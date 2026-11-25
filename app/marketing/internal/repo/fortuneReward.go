package repo

import (
	"encoding/json"
	"time"

	"gacha_srv/app/marketing/marketing"
	"gorm.io/gorm"
)

// FortuneRewardRow fortune_rewards 表行（吉祥值奖励配置）。
type FortuneRewardRow struct {
	ID             int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Name           string     `gorm:"column:name"`
	ExchangePoints int64      `gorm:"column:exchange_points"`
	DirectBuyMinor int64      `gorm:"column:direct_buy_minor"`
	Stock          int32      `gorm:"column:stock"`
	StartAt        *time.Time `gorm:"column:start_at"`
	EndAt          *time.Time `gorm:"column:end_at"`
	CashbackMinor  int64      `gorm:"column:cashback_minor"`
	RewardsJSON    string     `gorm:"column:rewards_json"`
	CoverURL       string     `gorm:"column:cover_url"`
	Tag            string     `gorm:"column:tag"`
	Enabled        int32      `gorm:"column:enabled"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (FortuneRewardRow) TableName() string { return "fortune_rewards" }

func ListFortuneRewards(tx *gorm.DB, offset, limit int) ([]FortuneRewardRow, int64, error) {
	var total int64
	if err := tx.Model(&FortuneRewardRow{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []FortuneRewardRow
	if err := tx.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func GetFortuneReward(tx *gorm.DB, id int64) (*FortuneRewardRow, error) {
	var row FortuneRewardRow
	if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func InsertFortuneReward(tx *gorm.DB, row *FortuneRewardRow) error {
	return tx.Create(row).Error
}

func UpdateFortuneReward(tx *gorm.DB, row *FortuneRewardRow) error {
	return tx.Model(&FortuneRewardRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"name":             row.Name,
		"exchange_points":  row.ExchangePoints,
		"direct_buy_minor": row.DirectBuyMinor,
		"stock":            row.Stock,
		"start_at":         row.StartAt,
		"end_at":           row.EndAt,
		"cashback_minor":   row.CashbackMinor,
		"rewards_json":     row.RewardsJSON,
		"cover_url":        row.CoverURL,
		"tag":              row.Tag,
		"enabled":          row.Enabled,
	}).Error
}

// MarshalFortuneRewardItems 奖励列表 → JSON 快照。
func MarshalFortuneRewardItems(items []*marketing.FortuneRewardItem) string {
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// UnmarshalFortuneRewardItems JSON 快照 → 奖励列表；损坏时返回空列表。
func UnmarshalFortuneRewardItems(raw string) []*marketing.FortuneRewardItem {
	var items []*marketing.FortuneRewardItem
	if raw == "" {
		return items
	}
	_ = json.Unmarshal([]byte(raw), &items)
	return items
}

// ToProto 配置行 → console FortuneReward。
func (r *FortuneRewardRow) ToProto() *marketing.FortuneReward {
	startAt := int64(0)
	if r.StartAt != nil {
		startAt = r.StartAt.UnixMilli()
	}
	endAt := int64(0)
	if r.EndAt != nil {
		endAt = r.EndAt.UnixMilli()
	}
	return &marketing.FortuneReward{
		Id:                  uint64(r.ID),
		Name:                r.Name,
		ExchangePoints:      r.ExchangePoints,
		DirectBuyMinor:      r.DirectBuyMinor,
		Stock:               r.Stock,
		StartAtUnixMillis:   startAt,
		EndAtUnixMillis:     endAt,
		CashbackMinor:       r.CashbackMinor,
		Rewards:             UnmarshalFortuneRewardItems(r.RewardsJSON),
		CoverUrl:            r.CoverURL,
		Tag:                 r.Tag,
		Enabled:             r.Enabled == 1,
		CreatedAtUnixMillis: timeToUnixMillis(r.CreatedAt),
		UpdatedAtUnixMillis: timeToUnixMillis(r.UpdatedAt),
	}
}

func unixMillisTime(ms int64) *time.Time {
	if ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms)
	return &t
}

// FortuneRewardFromProto proto → 表行（创建/编辑共用）。
func FortuneRewardFromProto(r *marketing.FortuneReward) *FortuneRewardRow {
	if r == nil {
		return nil
	}
	return &FortuneRewardRow{
		ID:             int64(r.Id),
		Name:           r.Name,
		ExchangePoints: r.ExchangePoints,
		DirectBuyMinor: r.DirectBuyMinor,
		Stock:          r.Stock,
		StartAt:        unixMillisTime(r.StartAtUnixMillis),
		EndAt:          unixMillisTime(r.EndAtUnixMillis),
		CashbackMinor:  r.CashbackMinor,
		RewardsJSON:    MarshalFortuneRewardItems(r.Rewards),
		CoverURL:       r.CoverUrl,
		Tag:            r.Tag,
		Enabled:        boolToInt32(r.Enabled),
	}
}

func boolToInt32(v bool) int32 {
	if v {
		return 1
	}
	return 0
}
