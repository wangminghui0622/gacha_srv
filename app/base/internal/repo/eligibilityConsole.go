// Package repo M14d 控制台数据访问（base 域）：条件政策版本化（head+versions+rules）
// + 风控案件状态机（cases+events）。
// 事务由 logic 层开启并传入 tx；本包只做单表行级操作（M14b draw/repo/pool.go 先例）。
package repo

import (
	"time"

	"gorm.io/gorm"
)

// 表行（console 表无 goctl 模型——行锁 + CAS 场景直用 GORM）
type (
	// EligibilityHead eligibility_policies 表行
	EligibilityHead struct {
		ID             int64  `gorm:"column:id;primaryKey;autoIncrement"`
		PolicyCode     string `gorm:"column:policy_code"`
		Name           string `gorm:"column:name"`
		CurrentVersion int64  `gorm:"column:current_version"` // 生效版本（0=未发布）
		LatestVersion  int64  `gorm:"column:latest_version"`
		CreatedAt      time.Time
		UpdatedAt      time.Time
	}

	// EligibilityVersionRow eligibility_policy_versions 表行
	EligibilityVersionRow struct {
		ID            int64  `gorm:"column:id;primaryKey;autoIncrement"`
		PolicyID      int64  `gorm:"column:policy_id"`
		Version       int64  `gorm:"column:version"`
		Scene         int32  `gorm:"column:scene"` // 1 DISCOVER 2 VIEW 3 DRAW 4 BOOK
		Status        int32  `gorm:"column:status"` // 1 DRAFT 2 PUBLISHED 3 RETIRED
		Combination   int32  `gorm:"column:combination"` // 1 ALL 2 ANY
		RequiredFacts string `gorm:"column:required_facts"` // JSON 数组
		ConfigHash    []byte `gorm:"column:config_hash"` // BINARY(32)
		PublishedAt   *time.Time
		CreatedAt     time.Time
	}

	// EligibilityRuleRow eligibility_policy_rules 表行
	EligibilityRuleRow struct {
		ID            int64 `gorm:"column:id;primaryKey;autoIncrement"`
		PolicyID      int64 `gorm:"column:policy_id"`
		PolicyVersion int64 `gorm:"column:policy_version"`
		RuleID        string `gorm:"column:rule_id"`
		Expression    string
		FailureCode   string
		SortOrder     int32 `gorm:"column:sort_order"`
	}
)

func (EligibilityHead) TableName() string        { return "eligibility_policies" }
func (EligibilityVersionRow) TableName() string  { return "eligibility_policy_versions" }
func (EligibilityRuleRow) TableName() string     { return "eligibility_policy_rules" }

// GetEligibilityHeadByCode 按政策编码查头（无锁，CreateDraft 存在性检查用）
func GetEligibilityHeadByCode(tx *gorm.DB, code string) (*EligibilityHead, error) {
	var head EligibilityHead
	err := tx.Where("policy_code = ?", code).First(&head).Error
	return &head, err
}

// GetEligibilityHead 按 id 查头（无锁）
func GetEligibilityHead(tx *gorm.DB, id int64) (*EligibilityHead, error) {
	var head EligibilityHead
	err := tx.Where("id = ?", id).First(&head).Error
	return &head, err
}

// LockEligibilityHead 行锁头（FOR UPDATE；CreateRevision/Publish 串行化）；未找到返回 gorm.ErrRecordNotFound。
func LockEligibilityHead(tx *gorm.DB, id int64) (*EligibilityHead, error) {
	var head EligibilityHead
	res := tx.Raw(
		"SELECT id, policy_code, name, current_version, latest_version, created_at, updated_at "+
			"FROM eligibility_policies WHERE id = ? FOR UPDATE", id).Scan(&head)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &head, nil
}

// InsertEligibilityHead 新建政策头
func InsertEligibilityHead(tx *gorm.DB, head *EligibilityHead) error {
	return tx.Create(head).Error
}

// UpdateEligibilityHead 更新政策头（latest_version 前进 / 名称同步 / current_version 发布指针）
func UpdateEligibilityHead(tx *gorm.DB, head *EligibilityHead) error {
	return tx.Model(&EligibilityHead{}).Where("id = ?", head.ID).
		Updates(map[string]any{
			"name":            head.Name,
			"current_version": head.CurrentVersion,
			"latest_version":  head.LatestVersion,
		}).Error
}

// GetEligibilityVersion 按 (policy_id, version) 查版本行（无锁）
func GetEligibilityVersion(tx *gorm.DB, policyID, version int64) (*EligibilityVersionRow, error) {
	var row EligibilityVersionRow
	err := tx.Where("policy_id = ? AND version = ?", policyID, version).First(&row).Error
	return &row, err
}

// LockEligibilityVersion 行锁版本行（FOR UPDATE；Publish 串行化）；未找到返回 gorm.ErrRecordNotFound。
func LockEligibilityVersion(tx *gorm.DB, policyID, version int64) (*EligibilityVersionRow, error) {
	var row EligibilityVersionRow
	res := tx.Raw(
		"SELECT id, policy_id, version, scene, status, combination, required_facts, config_hash, "+
			"published_at, created_at FROM eligibility_policy_versions "+
			"WHERE policy_id = ? AND version = ? FOR UPDATE", policyID, version).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// GetLatestEligibilityVersion 最新版本行（无锁，ListPolicies/GetPolicy 用）
func GetLatestEligibilityVersion(tx *gorm.DB, policyID int64) (*EligibilityVersionRow, error) {
	var row EligibilityVersionRow
	err := tx.Where("policy_id = ?", policyID).
		Order("version DESC").Limit(1).First(&row).Error
	return &row, err
}

// InsertEligibilityVersion 插版本行（DRAFT；uk(policy_id,config_hash) 防同内容重复版本）
func InsertEligibilityVersion(tx *gorm.DB, row *EligibilityVersionRow) error {
	return tx.Create(row).Error
}

// PublishEligibilityVersion 发布：同 policy 下旧 PUBLISHED 全部 RETIRE，本行置 PUBLISHED+published_at
func PublishEligibilityVersion(tx *gorm.DB, row *EligibilityVersionRow, publishedAt time.Time) error {
	if err := tx.Model(&EligibilityVersionRow{}).
		Where("policy_id = ? AND status = ?", row.PolicyID, PolicyStatusPublished).
		Update("status", PolicyStatusRetired).Error; err != nil {
		return err
	}
	return tx.Model(&EligibilityVersionRow{}).Where("id = ?", row.ID).
		Updates(map[string]any{"status": PolicyStatusPublished, "published_at": publishedAt}).Error
}

// InsertEligibilityRules 插规则行（版本创建时整批写入）
func InsertEligibilityRules(tx *gorm.DB, rows []EligibilityRuleRow) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

// ListEligibilityRules 按 (policy_id, version) 查规则（sort_order 升序）
func ListEligibilityRules(tx *gorm.DB, policyID, version int64) ([]EligibilityRuleRow, error) {
	var rows []EligibilityRuleRow
	err := tx.Where("policy_id = ? AND policy_version = ?", policyID, version).
		Order("sort_order").Find(&rows).Error
	return rows, err
}

// 版本状态常量（对齐 design D3）
const (
	PolicyStatusDraft     int32 = 1
	PolicyStatusPublished int32 = 2
	PolicyStatusRetired   int32 = 3
)
