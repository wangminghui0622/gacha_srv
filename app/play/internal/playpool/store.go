// Package playpool 提供玩法池的公共能力：索引、状态、历史、幂等。
// 每个玩法的配置/奖池表由玩法自己维护，但结构统一为：
//
//	play_<mode>_pools
//	play_<mode>_prizes
package playpool

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 池状态。
const (
	StatusDraft   int64 = 1
	StatusLive    int64 = 2
	StatusOffline int64 = 3
	StatusVoid    int64 = 4
)

var validModes = map[string]bool{
	"memorymatch": true,
	"gashapon":    true,
	"fullcar":     true,
	"combochain":  true,
	"dig":         true,
	"groupbuy":    true,
}

// ValidMode 校验玩法代码，避免表名拼接被注入。
func ValidMode(mode string) bool { return validModes[mode] }

func poolTable(mode string) string  { return "play_" + mode + "_pools" }
func prizeTable(mode string) string { return "play_" + mode + "_prizes" }

// CatalogRow 公共索引。
type CatalogRow struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Mode        string     `gorm:"column:mode"`
	Name        string     `gorm:"column:name"`
	Status      int64      `gorm:"column:status"`
	PriceFen    int64      `gorm:"column:price_fen"`
	IPID        int64      `gorm:"column:ip_id"`
	TagsJSON    string     `gorm:"column:tags_json"`
	Weight      int        `gorm:"column:weight"`
	AdminJSON   string     `gorm:"column:admin_json"`
	VoidReason  string     `gorm:"column:void_reason"`
	VoidedAt    *time.Time `gorm:"column:voided_at"`
	VoidedBy    int64      `gorm:"column:voided_by"`
	CreatedBy   int64      `gorm:"column:created_by"`
	UpdatedBy   int64      `gorm:"column:updated_by"`
	PublishedAt *time.Time `gorm:"column:published_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (CatalogRow) TableName() string { return "play_pool_catalog" }

// PoolRow 每个玩法的当前配置行。
type PoolRow struct {
	PoolID     int64     `gorm:"column:pool_id;primaryKey"`
	PriceFen   int64     `gorm:"column:price_fen"`
	ConfigJSON string    `gorm:"column:config_json"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

// PrizeRow 每个玩法的当前奖池行。
type PrizeRow struct {
	ID               int64     `gorm:"column:id;primaryKey;autoIncrement"`
	PoolID           int64     `gorm:"column:pool_id"`
	PrizeIndex       int       `gorm:"column:prize_index"`
	Name             string    `gorm:"column:name"`
	PrizeGrade       string    `gorm:"column:prize_grade"`
	PrizeType        int       `gorm:"column:prize_type"`
	SkuID            int64     `gorm:"column:sku_id"`
	ChildPoolID      int64     `gorm:"column:child_pool_id"`
	CashFen          int64     `gorm:"column:cash_fen"`
	PointsAmount     int64     `gorm:"column:points_amount"`
	CouponTemplateNo string    `gorm:"column:coupon_template_no"`
	Weight           int64     `gorm:"column:weight"`
	Quantity         int       `gorm:"column:quantity"`
	IsPrize          int       `gorm:"column:is_prize"`
	ImageURL         string    `gorm:"column:image_url"`
	CostFen          int64     `gorm:"column:cost_fen"`
	SpecialKind      string    `gorm:"column:special_kind"`
	TargetValue      int       `gorm:"column:target_value"`
	ParentIndex      int       `gorm:"column:parent_index"`
	TotalPairCount   int       `gorm:"column:total_pair_count"`
	ExtraJSON        string    `gorm:"column:extra_json"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

// HistoryRow 修改历史。
type HistoryRow struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	PoolID     int64     `gorm:"column:pool_id"`
	Mode       string    `gorm:"column:mode"`
	Action     string    `gorm:"column:action"`
	BeforeJSON string    `gorm:"column:before_json"`
	AfterJSON  string    `gorm:"column:after_json"`
	Reason     string    `gorm:"column:reason"`
	OperatorID int64     `gorm:"column:operator_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (HistoryRow) TableName() string { return "play_pool_history" }

// RequestRow 创建/修改幂等。
type RequestRow struct {
	IdempotencyKey string    `gorm:"column:idempotency_key;primaryKey"`
	PoolID         int64     `gorm:"column:pool_id"`
	RequestHash    string    `gorm:"column:request_hash"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (RequestRow) TableName() string { return "play_pool_requests" }

// Pool 领域对象。
type Pool struct {
	ID          int64
	Mode        string
	Name        string
	Status      int64
	PriceFen    int64
	IPID        int64
	TagsJSON    string
	Weight      int
	AdminJSON   string
	ConfigJSON  string
	Prizes      []*Prize
	PublishedAt *time.Time
	OperatorID  int64
}

// Prize 领域奖品。
type Prize struct {
	Index            int
	Name             string
	PrizeGrade       string
	PrizeType        int
	SkuID            int64
	ChildPoolID      int64
	CashFen          int64
	PointsAmount     int64
	CouponTemplateNo string
	Weight           int64
	Quantity         int
	IsPrize          bool
	ImageURL         string
	CostFen          int64
	SpecialKind      string
	TargetValue      int
	ParentIndex      int
	TotalPairCount   int
	ExtraJSON        string
}

func now() time.Time { return time.Now() }

func (p *Pool) normalize() {
	if p.TagsJSON == "" {
		p.TagsJSON = "[]"
	}
	if p.AdminJSON == "" {
		p.AdminJSON = "{}"
	}
	if p.ConfigJSON == "" {
		p.ConfigJSON = "{}"
	}
	if p.Status == 0 {
		p.Status = StatusDraft
	}
}

func toPoolRow(p *Pool) PoolRow {
	return PoolRow{
		PoolID:     p.ID,
		PriceFen:   p.PriceFen,
		ConfigJSON: p.ConfigJSON,
		CreatedAt:  now(),
		UpdatedAt:  now(),
	}
}

func toPrizeRows(p *Pool) []PrizeRow {
	rows := make([]PrizeRow, 0, len(p.Prizes))
	for _, prize := range p.Prizes {
		extra := prize.ExtraJSON
		if extra == "" {
			extra = "{}"
		}
		isPrize := 0
		if prize.IsPrize {
			isPrize = 1
		}
		rows = append(rows, PrizeRow{
			PoolID:           p.ID,
			PrizeIndex:       prize.Index,
			Name:             prize.Name,
			PrizeGrade:       prize.PrizeGrade,
			PrizeType:        prize.PrizeType,
			SkuID:            prize.SkuID,
			ChildPoolID:      prize.ChildPoolID,
			CashFen:          prize.CashFen,
			PointsAmount:     prize.PointsAmount,
			CouponTemplateNo: prize.CouponTemplateNo,
			Weight:           prize.Weight,
			Quantity:         prize.Quantity,
			IsPrize:          isPrize,
			ImageURL:         prize.ImageURL,
			CostFen:          prize.CostFen,
			SpecialKind:      prize.SpecialKind,
			TargetValue:      prize.TargetValue,
			ParentIndex:      prize.ParentIndex,
			TotalPairCount:   prize.TotalPairCount,
			ExtraJSON:        extra,
			CreatedAt:        now(),
			UpdatedAt:        now(),
		})
	}
	return rows
}

func fromPrizeRow(row PrizeRow) *Prize {
	extra := row.ExtraJSON
	if extra == "" {
		extra = "{}"
	}
	return &Prize{
		Index:            row.PrizeIndex,
		Name:             row.Name,
		PrizeGrade:       row.PrizeGrade,
		PrizeType:        row.PrizeType,
		SkuID:            row.SkuID,
		ChildPoolID:      row.ChildPoolID,
		CashFen:          row.CashFen,
		PointsAmount:     row.PointsAmount,
		CouponTemplateNo: row.CouponTemplateNo,
		Weight:           row.Weight,
		Quantity:         row.Quantity,
		IsPrize:          row.IsPrize == 1,
		ImageURL:         row.ImageURL,
		CostFen:          row.CostFen,
		SpecialKind:      row.SpecialKind,
		TargetValue:      row.TargetValue,
		ParentIndex:      row.ParentIndex,
		TotalPairCount:   row.TotalPairCount,
		ExtraJSON:        extra,
	}
}

// Create 在一个事务内写入索引、玩法配置、奖池和历史。
func Create(tx *gorm.DB, p *Pool) error {
	if !ValidMode(p.Mode) {
		return fmt.Errorf("未知玩法: %s", p.Mode)
	}
	p.normalize()
	p.Status = StatusDraft
	row := CatalogRow{
		Mode:      p.Mode,
		Name:      p.Name,
		Status:    p.Status,
		PriceFen:  p.PriceFen,
		IPID:      p.IPID,
		TagsJSON:  p.TagsJSON,
		Weight:    p.Weight,
		AdminJSON: p.AdminJSON,
		CreatedBy: p.OperatorID,
		UpdatedBy: p.OperatorID,
		CreatedAt: now(),
		UpdatedAt: now(),
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	p.ID = row.ID
	if err := tx.Table(poolTable(p.Mode)).Create(&PoolRow{
		PoolID: p.ID, PriceFen: p.PriceFen, ConfigJSON: p.ConfigJSON,
		CreatedAt: now(), UpdatedAt: now(),
	}).Error; err != nil {
		return err
	}
	if len(p.Prizes) > 0 {
		if err := tx.Table(prizeTable(p.Mode)).Create(toPrizeRows(p)).Error; err != nil {
			return err
		}
	}
	return WriteHistory(tx, p.ID, p.Mode, "CREATE", "", mustJSON(p), "", p.OperatorID)
}

// Update 更新当前配置；只允许 DRAFT/OFFLINE。
func Update(tx *gorm.DB, p *Pool) error {
	if !ValidMode(p.Mode) {
		return fmt.Errorf("未知玩法: %s", p.Mode)
	}
	var before CatalogRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", p.ID).First(&before).Error; err != nil {
		return err
	}
	if before.Mode != p.Mode {
		return errors.New("赏池玩法不一致")
	}
	if before.Status != StatusDraft && before.Status != StatusOffline {
		return errors.New("只有草稿或下架状态可编辑")
	}
	beforePool, err := Get(tx, p.ID)
	if err != nil {
		return err
	}
	p.normalize()
	if err := tx.Model(&CatalogRow{}).Where("id = ?", p.ID).Updates(map[string]any{
		"name":       p.Name,
		"price_fen":  p.PriceFen,
		"ip_id":      p.IPID,
		"tags_json":  p.TagsJSON,
		"weight":     p.Weight,
		"admin_json": p.AdminJSON,
		"updated_by": p.OperatorID,
		"updated_at": now(),
	}).Error; err != nil {
		return err
	}
	if err := tx.Table(poolTable(p.Mode)).Where("pool_id = ?", p.ID).
		Updates(map[string]any{"price_fen": p.PriceFen, "config_json": p.ConfigJSON, "updated_at": now()}).Error; err != nil {
		return err
	}
	if err := tx.Table(prizeTable(p.Mode)).Where("pool_id = ?", p.ID).Delete(&PrizeRow{}).Error; err != nil {
		return err
	}
	if len(p.Prizes) > 0 {
		if err := tx.Table(prizeTable(p.Mode)).Create(toPrizeRows(p)).Error; err != nil {
			return err
		}
	}
	return WriteHistory(tx, p.ID, p.Mode, "UPDATE", mustJSON(beforePool), mustJSON(p), "", p.OperatorID)
}

// Get 读取当前配置和奖池。
func Get(db *gorm.DB, poolID int64) (*Pool, error) {
	var row CatalogRow
	if err := db.Where("id = ?", poolID).First(&row).Error; err != nil {
		return nil, err
	}
	if !ValidMode(row.Mode) {
		return nil, fmt.Errorf("未知玩法: %s", row.Mode)
	}
	var cfg PoolRow
	if err := db.Table(poolTable(row.Mode)).Where("pool_id = ?", poolID).First(&cfg).Error; err != nil {
		return nil, err
	}
	var prizeRows []PrizeRow
	if err := db.Table(prizeTable(row.Mode)).Where("pool_id = ?", poolID).
		Order("prize_index ASC").Find(&prizeRows).Error; err != nil {
		return nil, err
	}
	p := &Pool{
		ID:          row.ID,
		Mode:        row.Mode,
		Name:        row.Name,
		Status:      row.Status,
		PriceFen:    cfg.PriceFen,
		IPID:        row.IPID,
		TagsJSON:    row.TagsJSON,
		Weight:      row.Weight,
		AdminJSON:   row.AdminJSON,
		ConfigJSON:  cfg.ConfigJSON,
		Prizes:      make([]*Prize, 0, len(prizeRows)),
		PublishedAt: row.PublishedAt,
	}
	for _, prizeRow := range prizeRows {
		p.Prizes = append(p.Prizes, fromPrizeRow(prizeRow))
	}
	return p, nil
}

// SetStatus 更新池状态；VOID 会写入作废信息。
func SetStatus(tx *gorm.DB, poolID int64, status int64, reason string, operatorID int64) error {
	var row CatalogRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", poolID).First(&row).Error; err != nil {
		return err
	}
	updates := map[string]any{
		"status":     status,
		"updated_by": operatorID,
		"updated_at": now(),
	}
	if status == StatusLive {
		updates["published_at"] = now()
	}
	if status == StatusVoid {
		updates["void_reason"] = reason
		updates["voided_at"] = now()
		updates["voided_by"] = operatorID
	}
	if err := tx.Model(&CatalogRow{}).Where("id = ?", poolID).Updates(updates).Error; err != nil {
		return err
	}
	action := map[int64]string{
		StatusLive: "RELIST", StatusOffline: "OFFLINE", StatusVoid: "VOID",
	}[status]
	if action == "" {
		action = "STATUS"
	}
	return WriteHistory(tx, poolID, row.Mode, action, "", "", reason, operatorID)
}

// WriteHistory 写一条历史记录。
func WriteHistory(tx *gorm.DB, poolID int64, mode, action, beforeJSON, afterJSON, reason string, operatorID int64) error {
	row := HistoryRow{
		PoolID:     poolID,
		Mode:       mode,
		Action:     action,
		BeforeJSON: beforeJSON,
		AfterJSON:  afterJSON,
		Reason:     reason,
		OperatorID: operatorID,
		CreatedAt:  now(),
	}
	return tx.Create(&row).Error
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// ListFilter 列表过滤条件。
type ListFilter struct {
	Mode     string
	Status   int64
	Keyword  string
	Page     int
	PageSize int
}

// List 分页列出索引。
func List(db *gorm.DB, f ListFilter) ([]*CatalogRow, int64, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	q := db.Model(&CatalogRow{})
	if f.Mode != "" {
		q = q.Where("mode = ?", f.Mode)
	}
	if f.Status > 0 {
		q = q.Where("status = ?", f.Status)
	}
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		q = q.Where("(name LIKE ? OR CAST(id AS CHAR) LIKE ?)", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []*CatalogRow
	if err := q.Order("weight DESC, id DESC").
		Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetRequest 读取幂等记录。
func GetRequest(db *gorm.DB, key string) (*RequestRow, error) {
	var row RequestRow
	if err := db.Where("idempotency_key = ?", key).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// PutRequest 写入幂等记录。
func PutRequest(tx *gorm.DB, key string, poolID int64, hash string) error {
	return tx.Create(&RequestRow{
		IdempotencyKey: key,
		PoolID:         poolID,
		RequestHash:    hash,
		CreatedAt:      now(),
	}).Error
}
