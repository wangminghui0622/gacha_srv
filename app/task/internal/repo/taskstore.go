package repo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/task/internal/core/task"
	taskpb "gacha_srv/app/task/task"

	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// M15c task 数据访问：gacha_game 8 表状态机（server_v2 data/state_machine.go + data/mysql.go
// 语义逐条移植）+ gacha_ops 跨库只读计划快照（server_v2 data/plan.go LoadPublishedPlan）。
// 行锁 FOR UPDATE + request_id/槽位幂等；全部 raw SQL（M14d 先例，事务由 logic 层开启传入 tx）。
// 老项目本仓偏离：无租户、无 fact_version/outbox（D6）、无奖励发放管线（D3）。

const (
	taskPlanHeadSingleton = 1
	taskPlanStatusPublish = 2
)

// LoadPublishedPlan 跨库只读 gacha_ops：task_plan_head.current_version →
// task_plan_version(status=2).config_blob → decode 校验 config_hash 重挂元数据。
// 计划不存在 → errorx.CodeTaskPlanNotFound（3035 家族段已登记）。
func LoadPublishedPlan(ctx context.Context, opsDB *gorm.DB) (*ops.TaskPlan, error) {
	var head struct {
		SingletonID    uint64
		CurrentVersion uint64
		LatestVersion  uint64
	}
	if err := opsDB.WithContext(ctx).Raw("SELECT `singleton_id`,`current_version`,`latest_version` FROM `task_plan_head` WHERE `singleton_id`=?", taskPlanHeadSingleton).Scan(&head).Error; err != nil {
		return nil, fmt.Errorf("read task plan head: %w", err)
	}
	if head.CurrentVersion == 0 {
		return nil, task.ErrTaskNotFound
	}
	var row struct {
		Version     uint64
		Status      uint8
		ConfigHash  string
		ConfigBlob  string
		CreatedAt   time.Time
		PublishedAt *time.Time
	}
	if err := opsDB.WithContext(ctx).Raw("SELECT `version`,`status`,`config_hash`,`config_blob`,`created_at`,`published_at` FROM `task_plan_version` WHERE `version`=?", head.CurrentVersion).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("read task plan version: %w", err)
	}
	if row.Version == 0 || row.Status != taskPlanStatusPublish {
		return nil, task.ErrTaskNotFound
	}
	plan := new(ops.TaskPlan)
	if err := proto.Unmarshal([]byte(row.ConfigBlob), plan); err != nil {
		return nil, fmt.Errorf("decode task plan %d: %w", row.Version, err)
	}
	hash, err := task.PlanHash(plan)
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

// LoadUserState 读用户全部进度/签到行（无锁只读；List 用）
func LoadUserState(ctx context.Context, db *gorm.DB, userID uint64) (task.UserState, error) {
	var progress []TaskProgressRow
	if err := db.WithContext(ctx).Where("`user_id` = ?", userID).Order("`task_id`").Find(&progress).Error; err != nil {
		return task.UserState{}, fmt.Errorf("load task progress: %w", err)
	}
	var signIns []TaskSignInRow
	if err := db.WithContext(ctx).Where("`user_id` = ?", userID).Order("`task_id`").Find(&signIns).Error; err != nil {
		return task.UserState{}, fmt.Errorf("load task sign-in: %w", err)
	}
	state := task.UserState{
		Progress: make([]task.ProgressRow, 0, len(progress)),
		SignIns:  make([]task.SignInRow, 0, len(signIns)),
	}
	for _, row := range progress {
		state.Progress = append(state.Progress, task.ProgressRow{UserID: row.UserID, TaskID: row.TaskID, Period: row.PeriodKey, Amount: row.Amount, Claimed: row.ClaimedCount})
	}
	for _, row := range signIns {
		state.SignIns = append(state.SignIns, task.SignInRow{UserID: row.UserID, TaskID: row.TaskID, LastDate: row.LastDate, Streak: row.Streak})
	}
	return state, nil
}

// LoadUserPassProgress 读用户通行证进度行（List 视图用；无行 → 空切片）
func LoadUserPassProgress(ctx context.Context, db *gorm.DB, userID uint64) ([]TaskPassProgressRow, error) {
	var rows []TaskPassProgressRow
	if err := db.WithContext(ctx).Where("`user_id` = ?", userID).Order("`pass_id`").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load pass progress: %w", err)
	}
	return rows, nil
}

// LoadUserMember 读用户会员行（无行 → 零值行，不报错；老项目 EnsureMember 不存在，
// 视图层面 0 spend/0 level 即未达最低档）
func LoadUserMember(ctx context.Context, db *gorm.DB, userID uint64) (TaskMemberRow, error) {
	var row TaskMemberRow
	if err := db.WithContext(ctx).Raw("SELECT `user_id`,`spend_minor`,`level` FROM `task_member` WHERE `user_id` = ?", userID).Scan(&row).Error; err != nil {
		return TaskMemberRow{}, fmt.Errorf("load member: %w", err)
	}
	return row, nil
}

// SaveApply 事件应用（server_v2 SaveApply）：task_event INSERT IGNORE 幂等去重 →
// 进度/通行证/会员增量。重复事件返回 Duplicate=true 并回读会员行。
func SaveApply(ctx context.Context, tx *gorm.DB, delta task.ApplyDelta) (task.ApplyResult, error) {
	now := time.Now()
	result := tx.WithContext(ctx).Exec("INSERT IGNORE INTO `task_event`(`event_id`,`created_at`) VALUES (?,?)", delta.EventID, now)
	if result.Error != nil {
		return task.ApplyResult{}, result.Error
	}
	if result.RowsAffected == 0 {
		res := task.ApplyResult{Duplicate: true}
		if delta.Member != nil {
			var row TaskMemberRow
			if err := tx.WithContext(ctx).Raw("SELECT `user_id`,`spend_minor`,`level` FROM `task_member` WHERE `user_id` = ?", delta.Member.UserID).Scan(&row).Error; err != nil {
				return task.ApplyResult{}, err
			}
			if row.UserID != 0 {
				res.Member = &task.MemberRow{UserID: row.UserID, SpendMinor: row.SpendMinor, Level: row.Level}
			}
		}
		return res, nil
	}
	for _, row := range delta.Progress {
		if err := tx.WithContext(ctx).Exec("INSERT INTO `task_progress`(`user_id`,`task_id`,`period_key`,`amount`,`claimed_count`,`updated_at`) VALUES (?,?,?,?,0,?) ON DUPLICATE KEY UPDATE `amount` = `amount` + VALUES(`amount`), `updated_at` = VALUES(`updated_at`)",
			row.UserID, row.TaskID, row.Period, row.Amount, now).Error; err != nil {
			return task.ApplyResult{}, err
		}
	}
	for _, row := range delta.Passes {
		if err := tx.WithContext(ctx).Exec("INSERT INTO `task_pass_progress`(`user_id`,`pass_id`,`xp`,`premium`,`updated_at`) VALUES (?,?,LEAST(?,?),0,?) ON DUPLICATE KEY UPDATE `xp` = LEAST(`xp` + VALUES(`xp`), ?), `updated_at` = VALUES(`updated_at`)",
			row.UserID, row.PassID, row.XP, row.MaxXP, now, row.MaxXP).Error; err != nil {
			return task.ApplyResult{}, err
		}
	}
	if delta.Member != nil {
		member, err := applyMember(ctx, tx, *delta.Member, now)
		if err != nil {
			return task.ApplyResult{}, err
		}
		return task.ApplyResult{Member: member}, nil
	}
	return task.ApplyResult{}, nil
}

// applyMember 会员 spend 累加（server_v2 mysql.go applyMember；无 fact_version/outbox，D6）：
// INSERT IGNORE 占位 → FOR UPDATE 重读 → 溢出守卫 → 乐观锁 UPDATE。
func applyMember(ctx context.Context, tx *gorm.DB, delta task.MemberDelta, now time.Time) (*task.MemberRow, error) {
	if delta.UserID == 0 || delta.SpendMinor <= 0 {
		return nil, errors.New("invalid membership spend delta")
	}
	if err := tx.WithContext(ctx).Exec("INSERT IGNORE INTO `task_member`(`user_id`,`spend_minor`,`level`,`updated_at`) VALUES (?,0,0,?)", delta.UserID, now).Error; err != nil {
		return nil, err
	}
	var current TaskMemberRow
	if err := tx.WithContext(ctx).Raw("SELECT `user_id`,`spend_minor`,`level` FROM `task_member` WHERE `user_id` = ? FOR UPDATE", delta.UserID).Scan(&current).Error; err != nil {
		return nil, err
	}
	if current.SpendMinor > math.MaxInt64-delta.SpendMinor {
		return nil, errors.New("member spend exhausted")
	}
	nextSpend := current.SpendMinor + delta.SpendMinor
	nextLevel := task.MemberLevelForTiers(delta.Tiers, nextSpend)
	result := tx.WithContext(ctx).Exec("UPDATE `task_member` SET `spend_minor` = ?, `level` = ?, `updated_at` = ? WHERE `user_id` = ? AND `spend_minor` = ? AND `level` = ?",
		nextSpend, nextLevel, now, delta.UserID, current.SpendMinor, current.Level)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("membership state conflict")
	}
	return &task.MemberRow{UserID: delta.UserID, SpendMinor: nextSpend, Level: nextLevel}, nil
}

// ClaimTask 任务领取（server_v2 ClaimTask）：progress 行锁 → request_id 重放 →
// CycleLimit/MaxUint32 守卫 → SIGN_IN 走签到行校验 → 插入 claim 行 + claimed_count 乐观锁。
func ClaimTask(ctx context.Context, tx *gorm.DB, cmd task.TaskClaimCommand) (task.Claim, error) {
	now := time.Now()
	progress, err := lockProgress(ctx, tx, cmd.UserID, cmd.Task.ID, cmd.Period)
	if err != nil {
		return task.Claim{}, err
	}
	var existing TaskClaimRow
	err = tx.WithContext(ctx).Raw("SELECT `request_id`,`user_id`,`task_id`,`reward_template_id`,`reward_count`,`claimed_count` FROM `task_claim` WHERE `request_id` = ? FOR UPDATE", cmd.RequestID).Scan(&existing).Error
	if err != nil {
		return task.Claim{}, err
	}
	if existing.RequestID != "" {
		if existing.UserID != cmd.UserID || existing.TaskID != cmd.Task.ID {
			return task.Claim{}, task.ErrInvalidTask
		}
		return task.Claim{RewardTemplateID: existing.RewardTemplateID, RewardCount: existing.RewardCount, Claimed: existing.ClaimedCount}, nil
	}
	if cmd.Task.CycleLimit > 0 && progress.ClaimedCount >= cmd.Task.CycleLimit {
		return task.Claim{}, task.ErrTaskFinished
	}
	if progress.ClaimedCount == math.MaxUint32 {
		return task.Claim{}, task.ErrTaskFinished
	}
	nextClaimed := progress.ClaimedCount + 1
	if cmd.Task.Metric == taskpb.TaskMetric_TASK_METRIC_SIGN_IN {
		var signed TaskSignInRow
		signErr := tx.WithContext(ctx).Raw("SELECT `user_id`,`task_id`,`last_date`,`streak` FROM `task_sign_in` WHERE `user_id` = ? AND `task_id` = ? FOR UPDATE", cmd.UserID, cmd.Task.ID).Scan(&signed).Error
		if signErr != nil || signed.LastDate != cmd.Today || int64(signed.Streak) < cmd.Task.Target {
			return task.Claim{}, task.ErrTaskNotReady
		}
	} else {
		if cmd.Task.Target > math.MaxInt64/int64(nextClaimed) || progress.Amount < cmd.Task.Target*int64(nextClaimed) {
			return task.Claim{}, task.ErrTaskNotReady
		}
	}
	claim := task.Claim{RewardTemplateID: cmd.Task.RewardTemplateID, RewardCount: task.RewardCount(cmd.Task.Reward, cmd.Task.RewardCount), Claimed: nextClaimed}
	if err := tx.WithContext(ctx).Exec("INSERT INTO `task_claim`(`request_id`,`user_id`,`task_id`,`reward_template_id`,`reward_count`,`claimed_count`,`created_at`) VALUES (?,?,?,?,?,?,?)",
		cmd.RequestID, cmd.UserID, cmd.Task.ID, claim.RewardTemplateID, claim.RewardCount, nextClaimed, now).Error; err != nil {
		return task.Claim{}, err
	}
	result := tx.WithContext(ctx).Exec("UPDATE `task_progress` SET `claimed_count` = ?, `updated_at` = ? WHERE `user_id` = ? AND `task_id` = ? AND `period_key` = ? AND `claimed_count` = ?",
		nextClaimed, now, cmd.UserID, cmd.Task.ID, cmd.Period, progress.ClaimedCount)
	if result.Error != nil {
		return task.Claim{}, result.Error
	}
	if result.RowsAffected != 1 {
		return task.Claim{}, errors.New("task claim state conflict")
	}
	return claim, nil
}

// SignInUser 签到（server_v2 SignInUser）：行锁 → 无行 INSERT IGNORE 占位重锁 →
// 同日/昨日连续判定 → MaxUint32 守卫 → last_date/streak 乐观锁 UPDATE。
func SignInUser(ctx context.Context, tx *gorm.DB, cmd task.SignInCommand) (task.SignInResult, error) {
	now := time.Now()
	var row TaskSignInRow
	err := tx.WithContext(ctx).Raw("SELECT `user_id`,`task_id`,`last_date`,`streak` FROM `task_sign_in` WHERE `user_id` = ? AND `task_id` = ? FOR UPDATE", cmd.UserID, cmd.TaskID).Scan(&row).Error
	if err != nil {
		return task.SignInResult{}, err
	}
	if row.UserID == 0 {
		if insErr := tx.WithContext(ctx).Exec("INSERT IGNORE INTO `task_sign_in`(`user_id`,`task_id`,`last_date`,`streak`,`updated_at`) VALUES (?,?,'',0,?)", cmd.UserID, cmd.TaskID, now).Error; insErr != nil {
			return task.SignInResult{}, insErr
		}
		if err = tx.WithContext(ctx).Raw("SELECT `user_id`,`task_id`,`last_date`,`streak` FROM `task_sign_in` WHERE `user_id` = ? AND `task_id` = ? FOR UPDATE", cmd.UserID, cmd.TaskID).Scan(&row).Error; err != nil {
			return task.SignInResult{}, err
		}
	}
	if row.LastDate == cmd.Today {
		return task.SignInResult{Streak: row.Streak, Date: cmd.Today, Already: true}, nil
	}
	streak := uint32(1)
	if row.LastDate == cmd.Previous && row.Streak > 0 {
		if row.Streak == math.MaxUint32 {
			return task.SignInResult{}, task.ErrTaskFinished
		}
		streak = row.Streak + 1
	}
	result := tx.WithContext(ctx).Exec("UPDATE `task_sign_in` SET `last_date` = ?, `streak` = ?, `updated_at` = ? WHERE `user_id` = ? AND `task_id` = ? AND `last_date` = ? AND `streak` = ?",
		cmd.Today, streak, now, cmd.UserID, cmd.TaskID, row.LastDate, row.Streak)
	if result.Error != nil {
		return task.SignInResult{}, result.Error
	}
	if result.RowsAffected != 1 {
		return task.SignInResult{}, errors.New("task sign-in state conflict")
	}
	return task.SignInResult{Streak: streak, Date: cmd.Today}, nil
}

// UnlockPassUser 解锁付费通行证（server_v2 UnlockPassUser）
func UnlockPassUser(ctx context.Context, tx *gorm.DB, userID, passID uint64) error {
	now := time.Now()
	var row TaskPassProgressRow
	err := tx.WithContext(ctx).Raw("SELECT `user_id`,`pass_id`,`xp`,`premium` FROM `task_pass_progress` WHERE `user_id` = ? AND `pass_id` = ? FOR UPDATE", userID, passID).Scan(&row).Error
	if err != nil {
		return err
	}
	if row.UserID == 0 {
		if insErr := tx.WithContext(ctx).Exec("INSERT IGNORE INTO `task_pass_progress`(`user_id`,`pass_id`,`xp`,`premium`,`updated_at`) VALUES (?,?,0,0,?)", userID, passID, now).Error; insErr != nil {
			return insErr
		}
	}
	return tx.WithContext(ctx).Exec("UPDATE `task_pass_progress` SET `premium` = 1, `updated_at` = ? WHERE `user_id` = ? AND `pass_id` = ?", now, userID, passID).Error
}

// ClaimPassReward 通行证奖励领取（server_v2 ClaimPassReward）：progress 行锁 →
// request_id 重放（异用户/赛季/槽位 → ErrInvalidTask）→ 等级守卫 → 槽位判重 →
// PREMIUM 线按 PAYMENT(premium 标记)/MEMBER(会员等级) 校验 → 插入 claim 行。
func ClaimPassReward(ctx context.Context, tx *gorm.DB, cmd task.PassClaimCommand) (task.PassClaim, error) {
	now := time.Now()
	var progress TaskPassProgressRow
	err := tx.WithContext(ctx).Raw("SELECT `user_id`,`pass_id`,`xp`,`premium` FROM `task_pass_progress` WHERE `user_id` = ? AND `pass_id` = ? FOR UPDATE", cmd.UserID, cmd.Season.ID).Scan(&progress).Error
	if err != nil {
		return task.PassClaim{}, err
	}
	if progress.UserID == 0 {
		if insErr := tx.WithContext(ctx).Exec("INSERT IGNORE INTO `task_pass_progress`(`user_id`,`pass_id`,`xp`,`premium`,`updated_at`) VALUES (?,?,0,0,?)", cmd.UserID, cmd.Season.ID, now).Error; insErr != nil {
			return task.PassClaim{}, insErr
		}
		if err = tx.WithContext(ctx).Raw("SELECT `user_id`,`pass_id`,`xp`,`premium` FROM `task_pass_progress` WHERE `user_id` = ? AND `pass_id` = ? FOR UPDATE", cmd.UserID, cmd.Season.ID).Scan(&progress).Error; err != nil {
			return task.PassClaim{}, err
		}
	}
	var existing TaskPassClaimRow
	err = tx.WithContext(ctx).Raw("SELECT `request_id`,`user_id`,`pass_id`,`level`,`track`,`reward_template_id`,`reward_count` FROM `task_pass_claim` WHERE `request_id` = ? FOR UPDATE", cmd.RequestID).Scan(&existing).Error
	if err != nil {
		return task.PassClaim{}, err
	}
	if existing.RequestID != "" {
		if existing.UserID != cmd.UserID || existing.PassID != cmd.Season.ID || existing.Level != cmd.Reward.Level || existing.Track != uint8(cmd.Reward.Track) {
			return task.PassClaim{}, task.ErrInvalidTask
		}
		return task.PassClaim{Level: existing.Level, Track: taskpb.PassTrack(existing.Track), RewardTemplateID: existing.RewardTemplateID, RewardCount: existing.RewardCount}, nil
	}
	if task.PassLevel(cmd.Season, progress.XP) < cmd.Reward.Level {
		return task.PassClaim{}, task.ErrTaskNotReady
	}
	var slot TaskPassClaimRow
	slotErr := tx.WithContext(ctx).Raw("SELECT `request_id` FROM `task_pass_claim` WHERE `user_id` = ? AND `pass_id` = ? AND `level` = ? AND `track` = ? FOR UPDATE",
		cmd.UserID, cmd.Season.ID, cmd.Reward.Level, uint8(cmd.Reward.Track)).Scan(&slot).Error
	if slotErr != nil {
		return task.PassClaim{}, slotErr
	}
	if slot.RequestID != "" {
		return task.PassClaim{}, task.ErrTaskFinished
	}
	if cmd.Reward.Track == taskpb.PassTrack_PASS_TRACK_PREMIUM {
		switch cmd.Season.PremiumLine {
		case taskpb.PassPremiumLine_PASS_PREMIUM_LINE_PAYMENT:
			if progress.Premium == 0 {
				return task.PassClaim{}, task.ErrTaskNotReady
			}
		case taskpb.PassPremiumLine_PASS_PREMIUM_LINE_MEMBER:
			var member TaskMemberRow
			memberErr := tx.WithContext(ctx).Raw("SELECT `user_id`,`spend_minor`,`level` FROM `task_member` WHERE `user_id` = ? FOR UPDATE", cmd.UserID).Scan(&member).Error
			if memberErr != nil || member.Level < cmd.Season.PremiumMemberLevel {
				return task.PassClaim{}, task.ErrTaskNotReady
			}
		default:
			return task.PassClaim{}, task.ErrTaskNotReady
		}
	}
	claim := task.PassClaim{Level: cmd.Reward.Level, Track: cmd.Reward.Track, RewardTemplateID: cmd.Reward.RewardTemplateID, RewardCount: task.RewardCount(cmd.Reward.Reward, cmd.Reward.RewardCount)}
	if err := tx.WithContext(ctx).Exec("INSERT INTO `task_pass_claim`(`request_id`,`user_id`,`pass_id`,`level`,`track`,`reward_template_id`,`reward_count`,`created_at`) VALUES (?,?,?,?,?,?,?,?)",
		cmd.RequestID, cmd.UserID, cmd.Season.ID, claim.Level, uint8(claim.Track), claim.RewardTemplateID, claim.RewardCount, now).Error; err != nil {
		return task.PassClaim{}, err
	}
	return claim, nil
}

// ClaimMemberReward 会员奖励领取（server_v2 ClaimMemberReward）：member 行锁
// （无行即 NotReady，无 EnsureMember——老项目语义）→ request_id 重放 → 档位判重 → 插入。
func ClaimMemberReward(ctx context.Context, tx *gorm.DB, cmd task.MemberClaimCommand) (task.Claim, error) {
	var member TaskMemberRow
	err := tx.WithContext(ctx).Raw("SELECT `user_id`,`spend_minor`,`level` FROM `task_member` WHERE `user_id` = ? FOR UPDATE", cmd.UserID).Scan(&member).Error
	if err != nil {
		return task.Claim{}, err
	}
	if member.UserID == 0 || member.SpendMinor < cmd.Tier.SpendMinor {
		return task.Claim{}, task.ErrTaskNotReady
	}
	var existing TaskMemberClaimRow
	err = tx.WithContext(ctx).Raw("SELECT `request_id`,`user_id`,`level`,`reward_template_id`,`reward_count` FROM `task_member_claim` WHERE `request_id` = ? FOR UPDATE", cmd.RequestID).Scan(&existing).Error
	if err != nil {
		return task.Claim{}, err
	}
	if existing.RequestID != "" {
		if existing.UserID != cmd.UserID || existing.Level != cmd.Tier.Level {
			return task.Claim{}, task.ErrInvalidTask
		}
		return task.Claim{RewardTemplateID: existing.RewardTemplateID, RewardCount: existing.RewardCount, Claimed: existing.Level}, nil
	}
	var dup TaskMemberClaimRow
	dupErr := tx.WithContext(ctx).Raw("SELECT `request_id` FROM `task_member_claim` WHERE `user_id` = ? AND `level` = ? FOR UPDATE", cmd.UserID, cmd.Tier.Level).Scan(&dup).Error
	if dupErr != nil {
		return task.Claim{}, dupErr
	}
	if dup.RequestID != "" {
		return task.Claim{}, task.ErrTaskFinished
	}
	claim := task.Claim{RewardTemplateID: cmd.Tier.RewardTemplateID, RewardCount: task.RewardCount(cmd.Tier.Reward, cmd.Tier.RewardCount), Claimed: cmd.Tier.Level}
	if err := tx.WithContext(ctx).Exec("INSERT INTO `task_member_claim`(`request_id`,`user_id`,`level`,`reward_template_id`,`reward_count`,`created_at`) VALUES (?,?,?,?,?,?)",
		cmd.RequestID, cmd.UserID, cmd.Tier.Level, claim.RewardTemplateID, claim.RewardCount, time.Now()).Error; err != nil {
		return task.Claim{}, err
	}
	return claim, nil
}

// lockProgress 进度行锁：无行先 INSERT IGNORE 占位再锁（server_v2 LockTaskProgress/EnsureTaskProgress）
func lockProgress(ctx context.Context, tx *gorm.DB, userID, taskID uint64, period string) (TaskProgressRow, error) {
	var row TaskProgressRow
	err := tx.WithContext(ctx).Raw("SELECT `user_id`,`task_id`,`period_key`,`amount`,`claimed_count` FROM `task_progress` WHERE `user_id` = ? AND `task_id` = ? AND `period_key` = ? FOR UPDATE",
		userID, taskID, period).Scan(&row).Error
	if err != nil {
		return TaskProgressRow{}, err
	}
	if row.UserID == 0 {
		if insErr := tx.WithContext(ctx).Exec("INSERT IGNORE INTO `task_progress`(`user_id`,`task_id`,`period_key`,`amount`,`claimed_count`,`updated_at`) VALUES (?,?,?,0,0,?)",
			userID, taskID, period, time.Now()).Error; insErr != nil {
			return TaskProgressRow{}, insErr
		}
		if err = tx.WithContext(ctx).Raw("SELECT `user_id`,`task_id`,`period_key`,`amount`,`claimed_count` FROM `task_progress` WHERE `user_id` = ? AND `task_id` = ? AND `period_key` = ? FOR UPDATE",
			userID, taskID, period).Scan(&row).Error; err != nil {
			return TaskProgressRow{}, err
		}
	}
	return row, nil
}
