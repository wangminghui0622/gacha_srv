package repo

import (
	"time"

	"gacha_srv/app/marketing/marketing"
	"gorm.io/gorm"
)

// M14d 券模板不可变版本（coupon_template_heads/versions，D5）+ 发券 + 积分默认值。
// 事务由 logic 层开启并传入 tx。

// 券模板 kind 映射（player coupon_templates.kind ↔ console CouponTemplateKind，与 server_v2 同值）
const (
	PlayerTplKindFixedMinor  int32 = 1
	PlayerTplKindPercentBps  int32 = 2
	PlayerTplKindFree        int32 = 3
	PlayerTplKindCustomMinor int32 = 4
	PlayerTplKindDrawReward  int32 = 5
)

type (
	// CouponTemplateHeadRow coupon_template_heads 表行
	CouponTemplateHeadRow struct {
		TemplateID  int64 `gorm:"column:template_id;primaryKey"`
		NextVersion int64 `gorm:"column:next_version"`
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	// CouponTemplateVersionRow coupon_template_versions 表行（不可变）
	CouponTemplateVersionRow struct {
		ID                int64  `gorm:"column:id;primaryKey;autoIncrement"`
		TemplateID        int64  `gorm:"column:template_id"`
		Version           int64  `gorm:"column:version"`
		Kind              int32  `gorm:"column:kind"`
		AmountMinor       int64  `gorm:"column:amount_minor"`
		PercentBps        int32  `gorm:"column:percent_bps"`
		MinGrossMinor     int64  `gorm:"column:min_gross_minor"`
		Currency          string `gorm:"column:currency"`
		Funder            int32  `gorm:"column:funder"`
		Scene             int32  `gorm:"column:scene"`
		MinUnitPriceMinor int64  `gorm:"column:min_unit_price_minor"`
		MaxUnitPriceMinor int64  `gorm:"column:max_unit_price_minor"`
		MinItemCount      int32  `gorm:"column:min_item_count"`
		MaxItemCount      int32  `gorm:"column:max_item_count"`
		TargetBusinessKey string `gorm:"column:target_business_key"`
		ValidityDays      int32  `gorm:"column:validity_days"`
		RewardTemplateID  int64  `gorm:"column:reward_template_id"`
		RewardCount       int32  `gorm:"column:reward_count"`
		RequestID         string `gorm:"column:request_id"`
		PublishedAt       time.Time
	}

	// PlayerCouponTemplateRow player coupon_templates 物化行（玩家折扣计算读该表）
	PlayerCouponTemplateRow struct {
		TemplateNo string
		Name       string
		Kind       int32
		Value      int64
		MinGross   int64
	}

	// UserCouponGrantRow user_coupons 铸券行（Id 由 Create 回填）
	UserCouponGrantRow struct {
		ID                int64      `gorm:"column:id;primaryKey;autoIncrement"`
		CouponNo          string     `gorm:"column:coupon_no"` // gc_<requestId>#<n>（uk 幂等）
		UserID            int64      `gorm:"column:user_id"`
		TemplateNo        string     `gorm:"column:template_no"`
		CustomAmountMinor int64      `gorm:"column:custom_amount_minor"`
		Status            int32      `gorm:"column:status"`
		ExpiresAt         *time.Time `gorm:"column:expires_at"`
	}

	// LoyaltyDefaultsRow loyalty_offset_config id=1（tenant 单租户固定 1）
	LoyaltyDefaultsRow struct {
		PointsPerYuan int64 `gorm:"column:points_per_yuan"`
		MaxRatioBps   int64 `gorm:"column:max_ratio_bps"`
	}
)

func (CouponTemplateHeadRow) TableName() string    { return "coupon_template_heads" }
func (CouponTemplateVersionRow) TableName() string { return "coupon_template_versions" }
func (UserCouponGrantRow) TableName() string       { return "user_coupons" }

// GetCouponTemplateHead 读 head（不加锁）；未找到返回 gorm.ErrRecordNotFound。
func GetCouponTemplateHead(tx *gorm.DB, templateID int64) (*CouponTemplateHeadRow, error) {
	var row CouponTemplateHeadRow
	if err := tx.Where("template_id = ?", templateID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// LockCouponTemplateHead 锁 head（FOR UPDATE）；未找到返回 gorm.ErrRecordNotFound。
func LockCouponTemplateHead(tx *gorm.DB, templateID int64) (*CouponTemplateHeadRow, error) {
	var row CouponTemplateHeadRow
	res := tx.Raw(
		"SELECT template_id, next_version, created_at, updated_at "+
			"FROM coupon_template_heads WHERE template_id = ? FOR UPDATE", templateID).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// EnsureCouponTemplateHead 确保 head 存在（INSERT IGNORE，无则建 next_version=1）。
func EnsureCouponTemplateHead(tx *gorm.DB, templateID int64) error {
	now := time.Now()
	return tx.Exec(
		"INSERT IGNORE INTO coupon_template_heads (template_id, next_version, created_at, updated_at) "+
			"VALUES (?, 1, ?, ?)", templateID, now, now).Error
}

// GetCouponTemplateVersionByRequestID 按发布请求号查版本行（幂等重放）；未找到返回 ErrRecordNotFound。
func GetCouponTemplateVersionByRequestID(tx *gorm.DB, requestID string) (*CouponTemplateVersionRow, error) {
	var row CouponTemplateVersionRow
	if err := tx.Where("request_id = ?", requestID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetLatestCouponTemplateVersion 读最新版本行；未找到返回 gorm.ErrRecordNotFound。
func GetLatestCouponTemplateVersion(tx *gorm.DB, templateID int64) (*CouponTemplateVersionRow, error) {
	var row CouponTemplateVersionRow
	if err := tx.Where("template_id = ?", templateID).
		Order("version DESC").Limit(1).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetCouponTemplateVersion 读指定版本行；未找到返回 gorm.ErrRecordNotFound。
func GetCouponTemplateVersion(tx *gorm.DB, templateID, version int64) (*CouponTemplateVersionRow, error) {
	var row CouponTemplateVersionRow
	if err := tx.Where("template_id = ? AND version = ?", templateID, version).
		First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// InsertCouponTemplateVersion 插入不可变版本行（uk request_id 冲突返回 gorm.ErrDuplicatedKey）。
func InsertCouponTemplateVersion(tx *gorm.DB, v *CouponTemplateVersionRow) error {
	return tx.Create(v).Error
}

// AdvanceCouponTemplateHeadCAS head next_version CAS 前进（期望不符返回 updated=false）。
func AdvanceCouponTemplateHeadCAS(tx *gorm.DB, templateID, expected, next int64) (bool, error) {
	res := tx.Exec(
		"UPDATE coupon_template_heads SET next_version = ? WHERE template_id = ? AND next_version = ?",
		next, templateID, expected)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ListCouponTemplateVersions 游标分页（(template_id, version) 复合游标，回全版本行）。
func ListCouponTemplateVersions(tx *gorm.DB, afterTemplateID, afterVersion int64, limit int) ([]CouponTemplateVersionRow, error) {
	var rows []CouponTemplateVersionRow
	if err := tx.Where(
		"(template_id > ?) OR (template_id = ? AND version > ?)", afterTemplateID, afterTemplateID, afterVersion,
	).Order("template_id ASC, version ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpsertPlayerCouponTemplate 物化 player coupon_templates 行（template_no uk；已存在则更新）。
func UpsertPlayerCouponTemplate(tx *gorm.DB, r *PlayerCouponTemplateRow) error {
	now := time.Now()
	return tx.Exec(
		"INSERT INTO coupon_templates (template_no, name, kind, value, min_gross, status, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, 1, ?, ?) "+
			"ON DUPLICATE KEY UPDATE name = VALUES(name), kind = VALUES(kind), "+
			"value = VALUES(value), min_gross = VALUES(min_gross), status = 1, updated_at = VALUES(updated_at)",
		r.TemplateNo, r.Name, r.Kind, r.Value, r.MinGross, now, now).Error
}

// GetPlayerCouponTemplateID 按 template_no 取 player 行主键（缓存失效用）；未找到返回 ErrRecordNotFound。
func GetPlayerCouponTemplateID(tx *gorm.DB, templateNo string) (int64, error) {
	var id int64
	res := tx.Raw(
		"SELECT id FROM coupon_templates WHERE template_no = ?", templateNo).Scan(&id)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return id, nil
}

// InsertUserCouponGrant 铸券（coupon_no uk 冲突返回 gorm.ErrDuplicatedKey → 幂等重放）。
// Id/Status 由本函数回填（autoIncrement；状态恒 AVAILABLE=1）。
func InsertUserCouponGrant(tx *gorm.DB, r *UserCouponGrantRow) error {
	r.Status = 1
	return tx.Create(r).Error
}

// ListUserCouponsByPrefix 按 coupon_no 前缀查用户券（幂等重放：gc_<requestId># 前缀），
// 按 coupon_no 升序（与铸券序号一致）。
func ListUserCouponsByPrefix(tx *gorm.DB, userID int64, prefix string) ([]UserCouponGrantRow, error) {
	var rows []UserCouponGrantRow
	err := tx.Where("user_id = ? AND coupon_no LIKE ?", userID, prefix+"%").
		Order("coupon_no ASC").Find(&rows).Error
	return rows, err
}

// GetLoyaltyDefaults 读积分默认值（loyalty_offset_config id=1）；未找到返回 ErrRecordNotFound。
func GetLoyaltyDefaults(tx *gorm.DB) (*LoyaltyDefaultsRow, error) {
	var row LoyaltyDefaultsRow
	res := tx.Raw(
		"SELECT points_per_yuan, max_ratio_bps FROM loyalty_offset_config WHERE id = 1").Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// PutLoyaltyDefaults 写积分默认值（id=1 行）。
func PutLoyaltyDefaults(tx *gorm.DB, pointsPerYuan, maxRatioBps int64) error {
	return tx.Exec(
		"UPDATE loyalty_offset_config SET points_per_yuan = ?, max_ratio_bps = ? WHERE id = 1",
		pointsPerYuan, maxRatioBps).Error
}

// ToProto 版本行 → console CouponTemplate（tenant_id 单租户固定 1，status 恒 PUBLISHED）。
func (r *CouponTemplateVersionRow) ToProto() *marketing.CouponTemplate {
	return &marketing.CouponTemplate{
		TemplateId:        uint64(r.TemplateID),
		Version:           uint64(r.Version),
		TenantId:          1,
		Status:            marketing.CouponTemplateStatus_COUPON_TEMPLATE_STATUS_PUBLISHED,
		Kind:              marketing.CouponTemplateKind(r.Kind),
		AmountMinor:       r.AmountMinor,
		MinGrossMinor:     r.MinGrossMinor,
		Currency:          r.Currency,
		Funder:            marketing.DiscountFunder(r.Funder),
		Scene:             marketing.CheckoutScene(r.Scene),
		PercentBps:        r.PercentBps,
		MinUnitPriceMinor: r.MinUnitPriceMinor,
		MaxUnitPriceMinor: r.MaxUnitPriceMinor,
		MinItemCount:      uint32(r.MinItemCount),
		MaxItemCount:      uint32(r.MaxItemCount),
		TargetBusinessKey: r.TargetBusinessKey,
		ValidityDays:      uint32(r.ValidityDays),
		RewardTemplateId:  uint64(r.RewardTemplateID),
		RewardCount:       uint32(r.RewardCount),
	}
}
