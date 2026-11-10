// 任务计划存储（M14e）：head 单行锁 + 版本表 + 发布审批表。
// 语义照抄 server_v2 task internal/data/plan.go（FOR UPDATE 串行化 + CAS）：
//   - CreateTaskPlanDraft：锁 head → latest==expectedLatestVersion 否则 CONFLICT →
//     INSERT DRAFT v=latest+1 → CAS head(latest→v)
//   - RequestTaskPlanPublication：同申请人+同哈希重放先前审批（任意状态）
//   - ApproveTaskPlanPublication：APPROVED 同审批人重放；自批 CONFLICT
//   - PublishTaskPlan：已发布同版本+哈希重放；structured reward → NOT_READY；
//     审批 APPROVED+审批人≠申请人 → RetirePublished(2→3) → publish CAS → head CAS
// 错误映射：sql.ErrNoRows→CodeTaskPlanNotFound；CAS 失败/冲突→CodeTaskPlanConflict；
// 校验→CodeTaskPlanInvalid；structured→CodeTaskPlanNotReady。
package repo

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"time"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

const (
	taskPlanHeadSingleton = 1
	taskPlanStatusDraft   = 1
	taskPlanStatusPublish = 2
	taskPlanStatusRetired = 3
	approvalStatusPending = 1
	approvalStatusApprove = 2
)

// PlanStore 任务计划数据访问（全部经 head 单行锁串行化）。
type PlanStore struct {
	db *gorm.DB
}

// NewPlanStore 构造任务计划存储。
func NewPlanStore(db *gorm.DB) *PlanStore {
	return &PlanStore{db: db}
}

// EnsureBootstrapPlan 幂等启动种子计划 v1（已发布），与 SQL 种子同构。
// 空库时插入：v1 status=2 + hash/blob（重算自 core.BootstrapTaskPlan）
// + CAS head(0,0 → 1,1)。
func (s *PlanStore) EnsureBootstrapPlan(ctx context.Context) error {
	hash, blob, err := core.TaskPlanPayload(core.BootstrapTaskPlan())
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := s.ensureHead(ctx, tx, now); err != nil {
			return fmt.Errorf("ensure task plan head: %w", err)
		}
		head, err := s.lockHead(ctx, tx)
		if err != nil {
			return fmt.Errorf("lock task plan head: %w", err)
		}
		if head.LatestVersion != 0 {
			return nil
		}
		if err := tx.Exec("INSERT INTO `task_plan_version` (`version`,`status`,`config_hash`,`config_blob`,`created_at`,`published_at`) VALUES (?,?,?,?,?,?)",
			1, taskPlanStatusPublish, string(hash), string(blob), now, now).Error; err != nil {
			return fmt.Errorf("insert bootstrap task plan: %w", err)
		}
		if err := s.casHead(ctx, tx, 1, 1, 0, 0); err != nil {
			return fmt.Errorf("publish bootstrap task plan: %w", err)
		}
		return nil
	})
}

// CreateTaskPlanDraft 创建草稿版本（expectedLatestVersion CAS 守卫）。
func (s *PlanStore) CreateTaskPlanDraft(ctx context.Context, plan *ops.TaskPlan, expectedLatestVersion uint64) (*ops.TaskPlan, error) {
	hash, blob, err := core.TaskPlanPayload(plan)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanInvalid, err.Error())
	}
	var result *ops.TaskPlan
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := s.ensureHead(ctx, tx, now); err != nil {
			return err
		}
		head, err := s.lockHead(ctx, tx)
		if err != nil {
			return err
		}
		if head.LatestVersion != expectedLatestVersion {
			return s.conflictf("expected_latest_version %d != 实际 latest %d", expectedLatestVersion, head.LatestVersion)
		}
		version := head.LatestVersion + 1
		if err := tx.Exec("INSERT INTO `task_plan_version` (`version`,`status`,`config_hash`,`config_blob`,`created_at`) VALUES (?,?,?,?,?)",
			version, taskPlanStatusDraft, string(hash), string(blob), now).Error; err != nil {
			return fmt.Errorf("insert task plan draft: %w", err)
		}
		if err := s.casHead(ctx, tx, head.CurrentVersion, version, head.CurrentVersion, head.LatestVersion); err != nil {
			return err
		}
		result = &ops.TaskPlan{
			Version: version, Status: ops.TaskPlanStatus_TASK_PLAN_STATUS_DRAFT,
			Tasks: plan.GetTasks(), Passes: plan.GetPasses(), MemberTiers: plan.GetMemberTiers(),
			ConfigHash: append([]byte(nil), hash...), CreatedAtUnixMillis: now.UnixMilli(),
		}
		return nil
	})
	return result, err
}

// GetTaskPlan 读取计划版本；version=0 取 latest。
func (s *PlanStore) GetTaskPlan(ctx context.Context, version uint64) (*ops.TaskPlan, error) {
	if version == 0 {
		head, err := s.getHead(ctx)
		if err != nil {
			return nil, fmt.Errorf("get task plan head: %w", err)
		}
		version = head.LatestVersion
	}
	if version == 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanNotFound, "任务计划不存在")
	}
	row, err := s.getVersion(ctx, version)
	if err != nil {
		return nil, err
	}
	return decodeTaskPlan(row)
}

// ListTaskPlans 分页列计划（pageSize 0/>100→20；peek 决定 next_after_version）。
func (s *PlanStore) ListTaskPlans(ctx context.Context, afterVersion uint64, pageSize uint32) ([]*ops.TaskPlan, uint64, error) {
	if pageSize == 0 || pageSize > 100 {
		pageSize = 20
	}
	var rows []opsmodel.TaskPlanVersion
	err := s.db.WithContext(ctx).Where("`version` > ?", afterVersion).
		Order("`version`").Limit(int(pageSize + 1)).Find(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list task plans: %w", err)
	}
	next := uint64(0)
	if len(rows) > int(pageSize) {
		next = rows[pageSize-1].Version
		rows = rows[:pageSize]
	}
	plans := make([]*ops.TaskPlan, 0, len(rows))
	for i := range rows {
		plan, decodeErr := decodeTaskPlan(&rows[i])
		if decodeErr != nil {
			return nil, 0, decodeErr
		}
		plans = append(plans, plan)
	}
	return plans, next, nil
}

// RequestTaskPlanPublication 提请发布审批（同申请人+同哈希重放先前审批，任意状态）。
func (s *PlanStore) RequestTaskPlanPublication(ctx context.Context, version uint64, expectedHash []byte, requesterID uint64) (*ops.TaskPlanPublicationApproval, error) {
	if version == 0 || requesterID == 0 || len(expectedHash) != 32 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanInvalid, "提请发布参数非法（version/requester/hash）")
	}
	var result *ops.TaskPlanPublicationApproval
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := s.lockHead(ctx, tx)
		if err != nil {
			return err
		}
		if head.LatestVersion != version {
			return s.conflictf("计划 %d 不是最新版本", version)
		}
		row, err := s.lockVersion(ctx, tx, version)
		if err != nil {
			return err
		}
		if row.Status != taskPlanStatusDraft || !bytes.Equal([]byte(row.ConfigHash), expectedHash) {
			return s.conflictf("计划 %d 非草稿或哈希不符", version)
		}
		prior, err := s.lockApproval(ctx, tx, version)
		if err == nil {
			if prior.RequesterId != requesterID || !bytes.Equal([]byte(prior.ConfigHash), expectedHash) {
				return s.conflictf("计划 %d 已有他人/异载荷审批", version)
			}
			result = approvalFromRow(prior)
			return nil
		}
		if !IsNotFound(err) {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Exec("INSERT INTO `task_plan_publication_approval` (`plan_version`,`config_hash`,`requester_id`,`approver_id`,`status`,`requested_at`) VALUES (?,?,?,0,?,?)",
			version, string(expectedHash), requesterID, approvalStatusPending, now).Error; err != nil {
			return err
		}
		result = &ops.TaskPlanPublicationApproval{
			Version: version, ConfigHash: append([]byte(nil), expectedHash...),
			RequesterId: requesterID, RequestedAtUnixMillis: now.UnixMilli(),
		}
		return nil
	})
	return result, err
}

// ApproveTaskPlanPublication 批准发布（自批拒绝；APPROVED 同审批人重放）。
func (s *PlanStore) ApproveTaskPlanPublication(ctx context.Context, version uint64, expectedHash []byte, approverID uint64) (*ops.TaskPlanPublicationApproval, error) {
	if version == 0 || approverID == 0 || len(expectedHash) != 32 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanInvalid, "批准参数非法（version/approver/hash）")
	}
	var result *ops.TaskPlanPublicationApproval
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := s.lockHead(ctx, tx)
		if err != nil {
			return err
		}
		if head.LatestVersion != version {
			return s.conflictf("计划 %d 不是最新版本", version)
		}
		row, err := s.lockVersion(ctx, tx, version)
		if err != nil {
			return err
		}
		if row.Status != taskPlanStatusDraft || !bytes.Equal([]byte(row.ConfigHash), expectedHash) {
			return s.conflictf("计划 %d 非草稿或哈希不符", version)
		}
		approval, err := s.lockApproval(ctx, tx, version)
		if err != nil {
			return s.mapNoRows(err, "发布审批不存在")
		}
		if !bytes.Equal([]byte(approval.ConfigHash), expectedHash) || approval.RequesterId == approverID {
			return s.conflictf("审批哈希不符或自批")
		}
		if approval.Status == approvalStatusApprove {
			if approval.ApproverId != approverID {
				return s.conflictf("计划 %d 已由他人批准", version)
			}
			result = approvalFromRow(approval)
			return nil
		}
		if approval.Status != approvalStatusPending {
			return s.conflictf("计划 %d 审批状态非法", version)
		}
		now := time.Now().UTC()
		res := tx.Exec("UPDATE `task_plan_publication_approval` SET `approver_id`=?,`status`=?,`approved_at`=? WHERE `plan_version`=? AND `config_hash`=?",
			approverID, approvalStatusApprove, now, version, string(expectedHash))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return s.conflictf("计划 %d 审批 CAS 失败", version)
		}
		approval.ApproverId = approverID
		approval.Status = approvalStatusApprove
		approval.ApprovedAt = &now
		result = approvalFromRow(approval)
		return nil
	})
	return result, err
}

// GetTaskPlanPublicationApproval 读取发布审批行。
func (s *PlanStore) GetTaskPlanPublicationApproval(ctx context.Context, version uint64) (*ops.TaskPlanPublicationApproval, error) {
	if version == 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanInvalid, "版本号非法")
	}
	row, err := s.getApproval(ctx, version)
	if err != nil {
		return nil, err
	}
	return approvalFromRow(row), nil
}

// PublishTaskPlan 发布计划（已发布同版本重放；structured reward → NOT_READY）。
func (s *PlanStore) PublishTaskPlan(ctx context.Context, version uint64, expectedHash []byte) (*ops.TaskPlan, error) {
	if version == 0 || len(expectedHash) != 32 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanInvalid, "发布参数非法（version/hash）")
	}
	var result *ops.TaskPlan
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := s.lockHead(ctx, tx)
		if err != nil {
			return err
		}
		if head.LatestVersion != version {
			return s.conflictf("计划 %d 不是最新版本", version)
		}
		row, err := s.lockVersion(ctx, tx, version)
		if err != nil {
			return err
		}
		if row.Status == taskPlanStatusPublish && head.CurrentVersion == version && bytes.Equal([]byte(row.ConfigHash), expectedHash) {
			result, err = decodeTaskPlan(row)
			return err
		}
		if row.Status != taskPlanStatusDraft || !bytes.Equal([]byte(row.ConfigHash), expectedHash) {
			return s.conflictf("计划 %d 非草稿或哈希不符", version)
		}
		definition, decodeErr := decodeTaskPlan(row)
		if decodeErr != nil {
			return decodeErr
		}
		if core.HasStructuredRewards(definition) {
			return errorx.NewCodeError(errorx.CodeTaskPlanNotReady, "计划含结构化奖励，原子发奖未就绪")
		}
		approval, err := s.lockApproval(ctx, tx, version)
		if err != nil {
			return s.mapNoRows(err, "发布审批不存在")
		}
		if approval.Status != approvalStatusApprove || approval.RequesterId == approval.ApproverId ||
			approval.ApproverId == 0 || !bytes.Equal([]byte(approval.ConfigHash), expectedHash) {
			return s.conflictf("计划 %d 审批未通过", version)
		}
		now := time.Now().UTC()
		if err := tx.Exec("UPDATE `task_plan_version` SET `status`=? WHERE `status`=?", taskPlanStatusRetired, taskPlanStatusPublish).Error; err != nil {
			return err
		}
		res := tx.Exec("UPDATE `task_plan_version` SET `status`=?,`published_at`=? WHERE `version`=? AND `status`=? AND `config_hash`=?",
			taskPlanStatusPublish, now, version, taskPlanStatusDraft, string(expectedHash))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return s.conflictf("计划 %d 发布 CAS 失败", version)
		}
		if err := s.casHead(ctx, tx, version, version, head.CurrentVersion, head.LatestVersion); err != nil {
			return err
		}
		row.Status = taskPlanStatusPublish
		row.PublishedAt = &now
		result, err = decodeTaskPlan(row)
		return err
	})
	return result, err
}

// ListPublishedTasks 已发布计划的全部任务（按 task_id 升序，server_v2 NewBoard 排序同款）。
// 无已发布计划 → CodeTaskPlanNotFound（server_v2 LoadPublishedPlan 同款）。
func (s *PlanStore) ListPublishedTasks(ctx context.Context) ([]*ops.TaskDefinition, error) {
	head, err := s.getHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("get task plan head: %w", err)
	}
	if head.CurrentVersion == 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanNotFound, "任务计划不存在")
	}
	plan, err := s.GetTaskPlan(ctx, head.CurrentVersion)
	if err != nil {
		return nil, err
	}
	tasks := append([]*ops.TaskDefinition(nil), plan.GetTasks()...)
	slicesSortTasks(tasks)
	return tasks, nil
}

// ── 行操作（FOR UPDATE / CAS）──

func (s *PlanStore) ensureHead(ctx context.Context, tx *gorm.DB, now time.Time) error {
	return tx.Exec("INSERT IGNORE INTO `task_plan_head` (`singleton_id`,`current_version`,`latest_version`,`updated_at`) VALUES (?,0,0,?)",
		taskPlanHeadSingleton, now).Error
}

func (s *PlanStore) lockHead(ctx context.Context, tx *gorm.DB) (*opsmodel.TaskPlanHead, error) {
	var head opsmodel.TaskPlanHead
	err := tx.Raw("SELECT `singleton_id`,`current_version`,`latest_version`,`updated_at` FROM `task_plan_head` WHERE `singleton_id`=? FOR UPDATE",
		taskPlanHeadSingleton).Scan(&head).Error
	if err != nil {
		return nil, err
	}
	if head.SingletonId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanNotFound, "任务计划 head 不存在")
	}
	return &head, nil
}

func (s *PlanStore) getHead(ctx context.Context) (*opsmodel.TaskPlanHead, error) {
	var head opsmodel.TaskPlanHead
	err := s.db.WithContext(ctx).Raw("SELECT `singleton_id`,`current_version`,`latest_version`,`updated_at` FROM `task_plan_head` WHERE `singleton_id`=?",
		taskPlanHeadSingleton).Scan(&head).Error
	if err != nil {
		return nil, err
	}
	if head.SingletonId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanNotFound, "任务计划 head 不存在")
	}
	return &head, nil
}

// casHead 单行 CAS：更新值需与 current/latest 现值一致。
func (s *PlanStore) casHead(ctx context.Context, tx *gorm.DB, currentVersion, latestVersion, expectedCurrent, expectedLatest uint64) error {
	res := tx.Exec("UPDATE `task_plan_head` SET `current_version`=?,`latest_version`=?,`updated_at`=? WHERE `singleton_id`=? AND `current_version`=? AND `latest_version`=?",
		currentVersion, latestVersion, time.Now().UTC(), taskPlanHeadSingleton, expectedCurrent, expectedLatest)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return s.conflictf("head CAS 失败（并发修改）")
	}
	return nil
}

func (s *PlanStore) lockVersion(ctx context.Context, tx *gorm.DB, version uint64) (*opsmodel.TaskPlanVersion, error) {
	var row opsmodel.TaskPlanVersion
	err := tx.Raw("SELECT * FROM `task_plan_version` WHERE `version`=? FOR UPDATE", version).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Version == 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanNotFound, fmt.Sprintf("任务计划 %d 不存在", version))
	}
	return &row, nil
}

func (s *PlanStore) getVersion(ctx context.Context, version uint64) (*opsmodel.TaskPlanVersion, error) {
	var row opsmodel.TaskPlanVersion
	err := s.db.WithContext(ctx).Raw("SELECT * FROM `task_plan_version` WHERE `version`=?", version).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Version == 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanNotFound, fmt.Sprintf("任务计划 %d 不存在", version))
	}
	return &row, nil
}

func (s *PlanStore) lockApproval(ctx context.Context, tx *gorm.DB, version uint64) (*opsmodel.TaskPlanPublicationApproval, error) {
	var row opsmodel.TaskPlanPublicationApproval
	err := tx.Raw("SELECT * FROM `task_plan_publication_approval` WHERE `plan_version`=? FOR UPDATE", version).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.PlanVersion == 0 {
		return nil, opsmodel.ErrNotFound
	}
	return &row, nil
}

func (s *PlanStore) getApproval(ctx context.Context, version uint64) (*opsmodel.TaskPlanPublicationApproval, error) {
	var row opsmodel.TaskPlanPublicationApproval
	err := s.db.WithContext(ctx).Raw("SELECT * FROM `task_plan_publication_approval` WHERE `plan_version`=?", version).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.PlanVersion == 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanNotFound, fmt.Sprintf("计划 %d 发布审批不存在", version))
	}
	return &row, nil
}

func (s *PlanStore) conflictf(format string, args ...any) error {
	logx.Infof("task plan conflict: "+format, args...)
	return errorx.NewCodeError(errorx.CodeTaskPlanConflict, fmt.Sprintf(format, args...))
}

func (s *PlanStore) mapNoRows(err error, msg string) error {
	if IsNotFound(err) {
		return errorx.NewCodeError(errorx.CodeTaskPlanNotFound, msg)
	}
	return err
}

// approvalFromRow 审批行 → proto（server_v2 approvalFromRow 同款）。
func approvalFromRow(row *opsmodel.TaskPlanPublicationApproval) *ops.TaskPlanPublicationApproval {
	approval := &ops.TaskPlanPublicationApproval{
		Version: row.PlanVersion, ConfigHash: append([]byte(nil), []byte(row.ConfigHash)...),
		RequesterId: row.RequesterId, ApproverId: row.ApproverId, Approved: row.Status == approvalStatusApprove,
		RequestedAtUnixMillis: row.RequestedAt.UTC().UnixMilli(),
	}
	if row.ApprovedAt != nil {
		approval.ApprovedAtUnixMillis = row.ApprovedAt.UTC().UnixMilli()
	}
	return approval
}

// decodeTaskPlan canonical blob → 完整计划（重挂版本/状态/时间戳，哈希重算校验）。
func decodeTaskPlan(row *opsmodel.TaskPlanVersion) (*ops.TaskPlan, error) {
	plan := new(ops.TaskPlan)
	if err := proto.Unmarshal([]byte(row.ConfigBlob), plan); err != nil {
		return nil, fmt.Errorf("decode task plan %d: %w", row.Version, err)
	}
	hash, err := core.TaskPlanConfigHash(plan)
	if err != nil || !bytes.Equal(hash, []byte(row.ConfigHash)) {
		return nil, fmt.Errorf("task plan %d payload hash mismatch", row.Version)
	}
	plan.Version = row.Version
	plan.Status = ops.TaskPlanStatus(row.Status)
	plan.ConfigHash = append([]byte(nil), []byte(row.ConfigHash)...)
	plan.CreatedAtUnixMillis = row.CreatedAt.UTC().UnixMilli()
	if row.PublishedAt != nil {
		plan.PublishedAtUnixMillis = row.PublishedAt.UTC().UnixMilli()
	}
	return plan, nil
}

// slicesSortTasks 按 task_id 升序（server_v2 NewBoard 排序同款）。
func slicesSortTasks(tasks []*ops.TaskDefinition) {
	slices.SortFunc(tasks, func(a, b *ops.TaskDefinition) int {
		switch {
		case a.GetTaskId() < b.GetTaskId():
			return -1
		case a.GetTaskId() > b.GetTaskId():
			return 1
		default:
			return 0
		}
	})
}
