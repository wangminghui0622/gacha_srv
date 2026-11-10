// Package repo 赏池控制台数据访问（M14b）：草稿/版本/审批四表，
// 行锁 + CAS 语义移植自 server_v2 gacha data/pool.go（mysql_pool 查询）。
// 事务由 logic 层开启并传入 tx；本包只做单表行级操作。
package repo

import (
	"time"

	"gorm.io/gorm"
)

// 表行（nocache，无 goctl 模型——复合主键 + 行锁场景直用 GORM）
type (
	PoolHead struct {
		PoolID      int64 `gorm:"column:pool_id;primaryKey;autoIncrement"`
		NextVersion int64 `gorm:"column:next_version"`
	}
	PoolVersion struct {
		PoolID      int64      `gorm:"column:pool_id;primaryKey"`
		Version     int64      `gorm:"column:version;primaryKey"`
		Status      uint8      `gorm:"column:status"`
		ConfigHash  []byte     `gorm:"column:config_hash"`
		ConfigJSON  string     `gorm:"column:config_json"`
		PublishedAt *time.Time `gorm:"column:published_at"`
		CreatedAt   time.Time  `gorm:"column:created_at"`
	}
	PoolDraftRequest struct {
		IdempotencyKey string    `gorm:"column:idempotency_key;primaryKey"`
		PoolID         int64     `gorm:"column:pool_id"`
		Version        int64     `gorm:"column:version"`
		RequestHash    []byte    `gorm:"column:request_hash"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}
	PoolPublicationApproval struct {
		PoolID      int64      `gorm:"column:pool_id;primaryKey"`
		Version     int64      `gorm:"column:version;primaryKey"`
		ConfigHash  []byte     `gorm:"column:config_hash"`
		RequesterID int64      `gorm:"column:requester_id"`
		ApproverID  int64      `gorm:"column:approver_id"`
		Status      uint8      `gorm:"column:status"`
		RequestedAt time.Time  `gorm:"column:requested_at"`
		ApprovedAt  *time.Time `gorm:"column:approved_at"`
	}
)

// 版本/审批状态（与 server_v2 同值）
const (
	StatusDraft     uint8 = 1
	StatusPublished uint8 = 2

	ApprovalPending  uint8 = 1
	ApprovalApproved uint8 = 2
)

func (PoolHead) TableName() string                { return "draw_pool_heads" }
func (PoolVersion) TableName() string             { return "draw_pool_versions" }
func (PoolDraftRequest) TableName() string        { return "draw_pool_draft_requests" }
func (PoolPublicationApproval) TableName() string { return "draw_pool_publication_approvals" }

// EnsurePoolHead 确保版本头存在（poolID=0 → 分配新池 id 并返回）。
func EnsurePoolHead(tx *gorm.DB, poolID int64) (int64, error) {
	if poolID != 0 {
		// INSERT IGNORE：已存在则不动（server_v2 EnsurePoolHead 同款语义）
		if err := tx.Exec(
			"INSERT IGNORE INTO draw_pool_heads (pool_id, next_version) VALUES (?, 1)", poolID,
		).Error; err != nil {
			return 0, err
		}
		return poolID, nil
	}
	head := &PoolHead{NextVersion: 1}
	if err := tx.Create(head).Error; err != nil {
		return 0, err
	}
	return head.PoolID, nil
}

// LockPoolHead 锁定版本头并返回当前 next_version（FOR UPDATE）。
func LockPoolHead(tx *gorm.DB, poolID int64) (int64, error) {
	var next int64
	if err := tx.Raw(
		"SELECT next_version FROM draw_pool_heads WHERE pool_id = ? FOR UPDATE", poolID,
	).Scan(&next).Error; err != nil {
		return 0, err
	}
	return next, nil
}

// AdvancePoolHead CAS 推进版本头（期望值不符返回 updated=false）。
func AdvancePoolHead(tx *gorm.DB, poolID, expected, next int64) (bool, error) {
	res := tx.Exec(
		"UPDATE draw_pool_heads SET next_version = ? WHERE pool_id = ? AND next_version = ?",
		next, poolID, expected)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// InsertPoolVersion 插入版本行（uk pool_config 冲突返回 gorm.ErrDuplicatedKey）。
func InsertPoolVersion(tx *gorm.DB, v *PoolVersion) error {
	return tx.Create(v).Error
}

// LockPoolVersion 锁定版本行（FOR UPDATE）；未找到返回 gorm.ErrRecordNotFound。
func LockPoolVersion(tx *gorm.DB, poolID, version int64) (*PoolVersion, error) {
	var row PoolVersion
	res := tx.Raw(
		"SELECT pool_id, version, status, config_hash, config_json, published_at, created_at "+
			"FROM draw_pool_versions WHERE pool_id = ? AND version = ? FOR UPDATE",
		poolID, version).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// GetPoolVersion 读取版本行（不加锁）。
func GetPoolVersion(tx *gorm.DB, poolID, version int64) (*PoolVersion, error) {
	var row PoolVersion
	if err := tx.Where("pool_id = ? AND version = ?", poolID, version).
		First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// MarkPoolVersionPublished CAS 标记已发布（期望 DRAFT + hash 一致）。
func MarkPoolVersionPublished(tx *gorm.DB, poolID, version int64, hash []byte, at time.Time) (bool, error) {
	res := tx.Exec(
		"UPDATE draw_pool_versions SET status = ?, published_at = ? "+
			"WHERE pool_id = ? AND version = ? AND status = ? AND config_hash = ?",
		StatusPublished, at, poolID, version, StatusDraft, hash)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ListPoolVersions 游标分页（(pool_id, version) 复合游标，含草稿与已发布）。
func ListPoolVersions(tx *gorm.DB, afterPoolID, afterVersion int64, limit int) ([]PoolVersion, error) {
	var rows []PoolVersion
	if err := tx.Where(
		"(pool_id > ?) OR (pool_id = ? AND version > ?)", afterPoolID, afterPoolID, afterVersion,
	).Order("pool_id ASC, version ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListPublishedPools 游标分页（仅已发布）。
func ListPublishedPools(tx *gorm.DB, afterPoolID, afterVersion int64, limit int) ([]PoolVersion, error) {
	var rows []PoolVersion
	if err := tx.Where(
		"status = ? AND ((pool_id > ?) OR (pool_id = ? AND version > ?))",
		StatusPublished, afterPoolID, afterPoolID, afterVersion,
	).Order("pool_id ASC, version ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetPoolDraftRequest 按幂等键查草稿创建记录；未找到返回 gorm.ErrRecordNotFound。
func GetPoolDraftRequest(tx *gorm.DB, key string) (*PoolDraftRequest, error) {
	var row PoolDraftRequest
	if err := tx.Where("idempotency_key = ?", key).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// InsertPoolDraftRequest 插入幂等记录（重复键返回 gorm.ErrDuplicatedKey）。
func InsertPoolDraftRequest(tx *gorm.DB, r *PoolDraftRequest) error {
	return tx.Create(r).Error
}

// LockPoolPublicationApproval 锁定审批行（FOR UPDATE）；未找到返回 gorm.ErrRecordNotFound。
func LockPoolPublicationApproval(tx *gorm.DB, poolID, version int64) (*PoolPublicationApproval, error) {
	var row PoolPublicationApproval
	res := tx.Raw(
		"SELECT pool_id, version, config_hash, requester_id, approver_id, status, requested_at, approved_at "+
			"FROM draw_pool_publication_approvals WHERE pool_id = ? AND version = ? FOR UPDATE",
		poolID, version).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// InsertPoolPublicationApproval 插入待审批行。
func InsertPoolPublicationApproval(tx *gorm.DB, a *PoolPublicationApproval) error {
	return tx.Create(a).Error
}

// ApprovePoolPublication CAS 批准（期望 PENDING + hash 一致）。
func ApprovePoolPublication(tx *gorm.DB, poolID, version int64, hash []byte, approverID int64, at time.Time) (bool, error) {
	res := tx.Exec(
		"UPDATE draw_pool_publication_approvals SET status = ?, approver_id = ?, approved_at = ? "+
			"WHERE pool_id = ? AND version = ? AND status = ? AND config_hash = ?",
		ApprovalApproved, approverID, at, poolID, version, ApprovalPending, hash)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}
