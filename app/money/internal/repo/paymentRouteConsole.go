// Package repo M14d 控制台数据访问（payment 域）：支付路由存管三表（D6，半成品无消费方）。
// 事务由 logic 层开启并传入 tx；本包只做单表行级操作（game repo/campaignConsole.go 先例）。
package repo

import (
	"time"

	"gorm.io/gorm"
)

// 表行（console 表无 goctl 模型——行锁 + CAS 场景直用 GORM）
type (
	// PaymentRoutePolicyRow payment_route_policies 表行
	PaymentRoutePolicyRow struct {
		ID                      int64  `gorm:"column:id;primaryKey;autoIncrement"`
		TenantID                int64  `gorm:"column:tenant_id"`
		Channel                 int32  `gorm:"column:channel"`
		Scene                   int32  `gorm:"column:scene"`
		Mode                    int32  `gorm:"column:mode"`
		PinnedProvider          int32  `gorm:"column:pinned_provider"`
		PinnedMerchantAccountID string `gorm:"column:pinned_merchant_account_id"`
		Version                 int64  `gorm:"column:version"`
		UpdatedBy               int64  `gorm:"column:updated_by"`
		CreatedAt               time.Time
		UpdatedAt               time.Time
	}

	// PaymentRouteCandidateRow payment_route_candidates 表行
	PaymentRouteCandidateRow struct {
		ID                int64  `gorm:"column:id;primaryKey;autoIncrement"`
		PolicyID          int64  `gorm:"column:policy_id"`
		Provider          int32  `gorm:"column:provider"`
		MerchantAccountID string `gorm:"column:merchant_account_id"`
		Priority          int32  `gorm:"column:priority"`
		Weight            int32  `gorm:"column:weight"`
		Enabled           bool   `gorm:"column:enabled"`
		CreatedAt         time.Time
		UpdatedAt         time.Time
	}

	// PaymentRouteAuditRow payment_route_audits 表行（old_snapshot 首建为 NULL）
	PaymentRouteAuditRow struct {
		ID             int64  `gorm:"column:id;primaryKey;autoIncrement"`
		PolicyID       int64  `gorm:"column:policy_id"`
		TenantID       int64  `gorm:"column:tenant_id"`
		OldVersion     int64  `gorm:"column:old_version"`
		NewVersion     int64  `gorm:"column:new_version"`
		OperatorUserID int64  `gorm:"column:operator_user_id"`
		OldSnapshot    *string `gorm:"column:old_snapshot"`
		NewSnapshot    string `gorm:"column:new_snapshot"`
		ChangedAt      time.Time
	}
)

func (PaymentRoutePolicyRow) TableName() string    { return "payment_route_policies" }
func (PaymentRouteCandidateRow) TableName() string { return "payment_route_candidates" }
func (PaymentRouteAuditRow) TableName() string     { return "payment_route_audits" }

// GetRoutePolicy 按 uk(tenant,channel,scene) 读策略行（不加锁）；未找到返回 gorm.ErrRecordNotFound。
func GetRoutePolicy(tx *gorm.DB, tenantID int64, channel, scene int32) (*PaymentRoutePolicyRow, error) {
	var row PaymentRoutePolicyRow
	if err := tx.Where("tenant_id = ? AND channel = ? AND scene = ?", tenantID, channel, scene).
		First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// LockRoutePolicy 锁定策略行（FOR UPDATE）；未找到返回 gorm.ErrRecordNotFound。
func LockRoutePolicy(tx *gorm.DB, tenantID int64, channel, scene int32) (*PaymentRoutePolicyRow, error) {
	var row PaymentRoutePolicyRow
	res := tx.Raw(
		"SELECT id, tenant_id, channel, scene, mode, pinned_provider, "+
			"pinned_merchant_account_id, version, updated_by, created_at, updated_at "+
			"FROM payment_route_policies WHERE tenant_id = ? AND channel = ? AND scene = ? FOR UPDATE",
		tenantID, channel, scene).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// InsertRoutePolicy 插入策略行（新建 version=1）。
func InsertRoutePolicy(tx *gorm.DB, row *PaymentRoutePolicyRow) error {
	return tx.Create(row).Error
}

// UpdateRoutePolicyCAS 按 (id, version) CAS 更新（version→newVersion，updated_by=operator）；
// 行不匹配返回 ok=false。version 列必变，RowsAffected==1 判定可靠。
func UpdateRoutePolicyCAS(tx *gorm.DB, id, oldVersion, newVersion int64,
	mode, pinnedProvider int32, pinnedMerchant string, operatorID int64) (bool, error) {
	res := tx.Model(&PaymentRoutePolicyRow{}).
		Where("id = ? AND version = ?", id, oldVersion).
		Updates(map[string]any{
			"mode":                       mode,
			"pinned_provider":            pinnedProvider,
			"pinned_merchant_account_id": pinnedMerchant,
			"version":                    newVersion,
			"updated_by":                 operatorID,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ListRouteCandidates 读策略全部候选（priority 小值优先，id 兜底稳定序）。
func ListRouteCandidates(tx *gorm.DB, policyID int64) ([]PaymentRouteCandidateRow, error) {
	var rows []PaymentRouteCandidateRow
	if err := tx.Where("policy_id = ?", policyID).
		Order("priority ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteRouteCandidates 删除策略全部候选（PUT 全量替换）。
func DeleteRouteCandidates(tx *gorm.DB, policyID int64) error {
	return tx.Where("policy_id = ?", policyID).Delete(&PaymentRouteCandidateRow{}).Error
}

// InsertRouteCandidates 批量插入候选（回填自增 ID）。
func InsertRouteCandidates(tx *gorm.DB, rows []PaymentRouteCandidateRow) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

// InsertRouteAudit 插入变更审计行。
func InsertRouteAudit(tx *gorm.DB, row *PaymentRouteAuditRow) error {
	return tx.Create(row).Error
}
