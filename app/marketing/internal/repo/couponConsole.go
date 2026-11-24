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
		Name              string `gorm:"column:name"`
		Description       string `gorm:"column:description"`
		NaturalDay        int32  `gorm:"column:natural_day"`
		Enabled           int32  `gorm:"column:enabled"`
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
		RequestID         string `gorm:"column:request_id"`
		PublishedAt       time.Time
	}

	// CouponPoolEntryRow coupon_pool_entries 表行（券池记录，独立于模板版本）
	CouponPoolEntryRow struct {
		ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
		TemplateID     int64     `gorm:"column:template_id"`
		Count          int32     `gorm:"column:count"`
		RemainingCount int32     `gorm:"column:remaining_count"`
		DrawCount      int32     `gorm:"column:draw_count"`
		StartAt        int64     `gorm:"column:start_at"`
		Enabled        int32     `gorm:"column:enabled"`
		CreatedAt      time.Time `gorm:"column:created_at"`
		UpdatedAt      time.Time `gorm:"column:updated_at"`
		TemplateName   string    `gorm:"column:template_name;->"`
	}

	// CouponLedgerRow coupon_ledger 表行（库存调整 + 用户券状态事件）
	CouponLedgerRow struct {
		ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
		EntryID      int64     `gorm:"column:entry_id"`
		UserCouponID int64     `gorm:"column:user_coupon_id"`
		UserID       int64     `gorm:"column:user_id"`
		EventType    string    `gorm:"column:event_type"`
		Delta        int32     `gorm:"column:delta"`
		BeforeCount  int32     `gorm:"column:before_count"`
		AfterCount   int32     `gorm:"column:after_count"`
		BeforeStatus int32     `gorm:"column:before_status"`
		AfterStatus  int32     `gorm:"column:after_status"`
		OperatorType string    `gorm:"column:operator_type"`
		OperatorID   int64     `gorm:"column:operator_id"`
		OrderID      string    `gorm:"column:order_id"`
		Remark       string    `gorm:"column:remark"`
		CreatedAt    time.Time `gorm:"column:created_at"`
	}

	// PlayerCouponTemplateRow player coupon_templates 物化行（玩家折扣计算读该表）
	PlayerCouponTemplateRow struct {
		TemplateNo string
		Name       string
		Kind       int32
		Value      int64
		MinGross   int64
		Status     int32
	}

	// UserCouponGrantRow user_coupons 铸券行（Id 由 Create 回填）
	UserCouponGrantRow struct {
		ID                int64      `gorm:"column:id;primaryKey;autoIncrement"`
		CouponNo          string     `gorm:"column:coupon_no"` // gc_<requestId>#<n>（uk 幂等）
		UserID            int64      `gorm:"column:user_id"`
		TemplateNo        string     `gorm:"column:template_no"`
		PoolEntryID       int64      `gorm:"column:pool_entry_id"`
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
func (CouponPoolEntryRow) TableName() string       { return "coupon_pool_entries" }
func (CouponLedgerRow) TableName() string          { return "coupon_ledger" }
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

// ListCouponPoolEntries 券池分页（id 倒序，关联引用模板的最新版本名称）。
func ListCouponPoolEntries(tx *gorm.DB, offset, limit int) ([]CouponPoolEntryRow, int64, error) {
	var total int64
	if err := tx.Model(&CouponPoolEntryRow{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []CouponPoolEntryRow
	err := tx.Table("coupon_pool_entries AS e").
		Select("e.id, e.template_id, e.count, e.remaining_count, e.draw_count, e.start_at, e.enabled, e.created_at, e.updated_at, " +
			"COALESCE(v.name, '') AS template_name").
		Joins("LEFT JOIN coupon_template_versions v ON v.template_id = e.template_id " +
			"AND v.version = (SELECT MAX(v2.version) FROM coupon_template_versions v2 WHERE v2.template_id = e.template_id)").
		Order("e.id DESC").Limit(limit).Offset(offset).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetCouponPoolEntry 读单条券池记录（含最新模板名称）；未找到返回 gorm.ErrRecordNotFound。
func GetCouponPoolEntry(tx *gorm.DB, id int64) (*CouponPoolEntryRow, error) {
	var row CouponPoolEntryRow
	err := tx.Table("coupon_pool_entries AS e").
		Select("e.id, e.template_id, e.count, e.remaining_count, e.draw_count, e.start_at, e.enabled, e.created_at, e.updated_at, "+
			"COALESCE(v.name, '') AS template_name").
		Joins("LEFT JOIN coupon_template_versions v ON v.template_id = e.template_id "+
			"AND v.version = (SELECT MAX(v2.version) FROM coupon_template_versions v2 WHERE v2.template_id = e.template_id)").
		Where("e.id = ?", id).Limit(1).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// InsertCouponPoolEntry 创建券池记录（ID 由自增回填）。
func InsertCouponPoolEntry(tx *gorm.DB, row *CouponPoolEntryRow) error {
	return tx.Create(row).Error
}

// UpdateCouponPoolEntry 编辑券池记录（库存由 AdjustCouponPoolEntry 单独调整）。
func UpdateCouponPoolEntry(tx *gorm.DB, row *CouponPoolEntryRow) error {
	return tx.Model(&CouponPoolEntryRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"template_id": row.TemplateID,
		"draw_count":  row.DrawCount,
		"start_at":    row.StartAt,
		"enabled":     row.Enabled,
	}).Error
}

// LockCouponPoolEntry 锁券池行（FOR UPDATE），用于管理员库存调整。
func LockCouponPoolEntry(tx *gorm.DB, id int64) (*CouponPoolEntryRow, error) {
	var row CouponPoolEntryRow
	res := tx.Raw(
		"SELECT id, template_id, count, remaining_count, draw_count, start_at, enabled, created_at, updated_at "+
			"FROM coupon_pool_entries WHERE id = ? FOR UPDATE", id).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 || row.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// UpdateCouponPoolRemaining 写入库存调整结果（调用方已持有行锁）。
func UpdateCouponPoolRemaining(tx *gorm.DB, id int64, remaining int32) error {
	return tx.Model(&CouponPoolEntryRow{}).Where("id = ?", id).
		Update("remaining_count", remaining).Error
}

// InsertCouponLedger 写券池/用户券变动流水。
func InsertCouponLedger(tx *gorm.DB, row *CouponLedgerRow) error {
	return tx.Create(row).Error
}

// ListCouponLedger 流水分页（id 倒序）。筛选条件为 0/空 时不过滤。
func ListCouponLedger(tx *gorm.DB, entryID, userCouponID, userID int64, orderID string, offset, limit int) ([]CouponLedgerRow, int64, error) {
	query := func() *gorm.DB {
		db := tx.Model(&CouponLedgerRow{})
		if entryID > 0 {
			db = db.Where("entry_id = ?", entryID)
		}
		if userCouponID > 0 {
			db = db.Where("user_coupon_id = ?", userCouponID)
		}
		if userID > 0 {
			db = db.Where("user_id = ?", userID)
		}
		if orderID != "" {
			db = db.Where("order_id = ?", orderID)
		}
		return db
	}
	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []CouponLedgerRow
	if err := query().Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// UpsertPlayerCouponTemplate 物化 player coupon_templates 行（template_no uk；已存在则更新）。
func UpsertPlayerCouponTemplate(tx *gorm.DB, r *PlayerCouponTemplateRow) error {
	now := time.Now()
	return tx.Exec(
		"INSERT INTO coupon_templates (template_no, name, kind, value, min_gross, status, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?) "+
			"ON DUPLICATE KEY UPDATE name = VALUES(name), kind = VALUES(kind), "+
			"value = VALUES(value), min_gross = VALUES(min_gross), status = VALUES(status), updated_at = VALUES(updated_at)",
		r.TemplateNo, r.Name, r.Kind, r.Value, r.MinGross, r.Status, now, now).Error
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
	status := marketing.CouponTemplateStatus_COUPON_TEMPLATE_STATUS_PUBLISHED
	if r.Enabled == 0 {
		status = marketing.CouponTemplateStatus_COUPON_TEMPLATE_STATUS_DISABLED
	}
	return &marketing.CouponTemplate{
		TemplateId:        uint64(r.TemplateID),
		Version:           uint64(r.Version),
		Name:              r.Name,
		Description:       r.Description,
		NaturalDay:        r.NaturalDay == 1,
		Enabled:           r.Enabled == 1,
		TenantId:          1,
		Status:            status,
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
	}
}

// ToProto 券池行 → console CouponPoolEntry。
func (r *CouponPoolEntryRow) ToProto() *marketing.CouponPoolEntry {
	return &marketing.CouponPoolEntry{
		Id:                  uint64(r.ID),
		TemplateId:          uint64(r.TemplateID),
		TemplateName:        r.TemplateName,
		Count:               uint32(r.Count),
		DrawCount:           uint32(r.DrawCount),
		StartAtUnixMillis:   r.StartAt,
		Enabled:             r.Enabled == 1,
		CreatedAtUnixMillis: timeToUnixMillis(r.CreatedAt),
		UpdatedAtUnixMillis: timeToUnixMillis(r.UpdatedAt),
		RemainingCount:      uint32(r.RemainingCount),
	}
}

// ToProto 流水行 → console CouponLedgerEntry。
func (r *CouponLedgerRow) ToProto() *marketing.CouponLedgerEntry {
	return &marketing.CouponLedgerEntry{
		Id:                  uint64(r.ID),
		EntryId:             uint64(r.EntryID),
		UserCouponId:        uint64(r.UserCouponID),
		UserId:              uint64(r.UserID),
		EventType:           r.EventType,
		Delta:               r.Delta,
		BeforeCount:         uint32(r.BeforeCount),
		AfterCount:          uint32(r.AfterCount),
		BeforeStatus:        r.BeforeStatus,
		AfterStatus:         r.AfterStatus,
		OperatorType:        r.OperatorType,
		OperatorId:          uint64(r.OperatorID),
		OrderId:             r.OrderID,
		Remark:              r.Remark,
		CreatedAtUnixMillis: timeToUnixMillis(r.CreatedAt),
	}
}

func timeToUnixMillis(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	return uint64(t.UnixMilli())
}
