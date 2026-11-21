package repo

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MemoryMatchGlobalRuleRow 对对碰全局玩法规则（固定 id=1）。
type MemoryMatchGlobalRuleRow struct {
	ID                   int64     `gorm:"column:id;primaryKey" json:"id"`
	Name                 string    `gorm:"column:name;size:64;not null" json:"name"`
	PairProbabilityMode  int64     `gorm:"column:pair_probability_mode;not null;default:0" json:"pairProbabilityMode"`
	InitialHandCount     int64     `gorm:"column:initial_hand_count;not null;default:9" json:"initialHandCount"`
	PairSize             int64     `gorm:"column:pair_size;not null;default:2" json:"pairSize"`
	ReplenishCount       int64     `gorm:"column:replenish_count;not null;default:1" json:"replenishCount"`
	SecondDestinyEnabled int64     `gorm:"column:second_destiny_enabled;not null;default:1" json:"secondDestinyEnabled"`
	DestinyDoubleEnabled int64     `gorm:"column:destiny_double_enabled;not null;default:1" json:"destinyDoubleEnabled"`
	DestinyIncome        int64     `gorm:"column:destiny_income;not null;default:1" json:"destinyIncome"`
	DoubleDestinyBuff    int64     `gorm:"column:double_destiny_buff;not null;default:1" json:"doubleDestinyBuff"`
	DestinyReplenishCost int64     `gorm:"column:destiny_replenish_cost;not null;default:1" json:"destinyReplenishCost"`
	ResumeEnabled        int64     `gorm:"column:resume_enabled;not null;default:1" json:"resumeEnabled"`
	HostModeEnabled      int64     `gorm:"column:host_mode_enabled;not null;default:1" json:"hostModeEnabled"`
	HostMinPairs         int64     `gorm:"column:host_min_pairs;not null;default:6" json:"hostMinPairs"`
	HostShareBps         int64     `gorm:"column:host_share_bps;not null;default:50" json:"hostShareBps"`
	RevenueBase          int64     `gorm:"column:revenue_base;not null;default:1" json:"revenueBase"`
	LedgerType           int64     `gorm:"column:ledger_type;not null;default:0" json:"ledgerType"`
	Status               int64     `gorm:"column:status;not null;default:1" json:"status"`
	CreatedAt            time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt            time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (MemoryMatchGlobalRuleRow) TableName() string { return "memory_match_global_rules" }

// MemoryMatchDeckTemplateRow 对对碰牌堆模板。
type MemoryMatchDeckTemplateRow struct {
	ID         int64     `gorm:"column:id;primaryKey" json:"id"`
	Name       string    `gorm:"column:name;size:64;not null" json:"name"`
	Status     int64     `gorm:"column:status;not null;default:1" json:"status"`
	TotalCards int64     `gorm:"column:total_cards;not null;default:54" json:"totalCards"`
	Weight     int64     `gorm:"column:weight;not null;default:0" json:"weight"`
	ConfigJSON string    `gorm:"column:config_json;type:json" json:"configJson"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (MemoryMatchDeckTemplateRow) TableName() string { return "memory_match_deck_templates" }

// MemoryMatchAppearanceRow 对对碰外观池。
type MemoryMatchAppearanceRow struct {
	ID             int64     `gorm:"column:id;primaryKey" json:"id"`
	AppearanceType int64     `gorm:"column:appearance_type;not null;default:1" json:"appearanceType"`
	Name           string    `gorm:"column:name;size:64;not null" json:"name"`
	ImageURL       string    `gorm:"column:image_url;size:512;not null;default:''" json:"imageUrl"`
	BackImageURL   string    `gorm:"column:back_image_url;size:512;not null;default:''" json:"backImageUrl"`
	Status         int64     `gorm:"column:status;not null;default:1" json:"status"`
	Free           int64     `gorm:"column:free;not null;default:1" json:"free"`
	ExchangePoints string    `gorm:"column:exchange_points;type:decimal(12,2);not null;default:0" json:"exchangePoints"`
	FunctionType   string    `gorm:"column:function_type;size:64;not null;default:''" json:"functionType"`
	FunctionConfig string    `gorm:"column:function_config;size:1024;not null;default:''" json:"functionConfig"`
	Weight         int64     `gorm:"column:weight;not null;default:0" json:"weight"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (MemoryMatchAppearanceRow) TableName() string { return "memory_match_appearances" }

// MemoryMatchAdminStore 对对碰管理后台存储。
type MemoryMatchAdminStore struct {
	db *gorm.DB
}

func NewMemoryMatchAdminStore(db *gorm.DB) *MemoryMatchAdminStore {
	return &MemoryMatchAdminStore{db: db}
}

// EnsureMemoryMatchAdminSeed 建表并写入默认规则/牌堆/外观。
// 正式环境也可直接执行 script/split/07_gacha_play.sql；这里保证 clone 后可直接使用。
func EnsureMemoryMatchAdminSeed(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&MemoryMatchGlobalRuleRow{},
		&MemoryMatchDeckTemplateRow{},
		&MemoryMatchAppearanceRow{},
	); err != nil {
		return err
	}

	var ruleCount int64
	if err := db.Model(&MemoryMatchGlobalRuleRow{}).Where("id = ?", 1).Count(&ruleCount).Error; err != nil {
		return err
	}
	if ruleCount == 0 {
		now := time.Now()
		rule := MemoryMatchGlobalRuleRow{
			ID:                   1,
			Name:                 "默认全局规则",
			PairProbabilityMode:  0,
			InitialHandCount:     9,
			PairSize:             2,
			ReplenishCount:       1,
			SecondDestinyEnabled: 1,
			DestinyDoubleEnabled: 1,
			DestinyIncome:        1,
			DoubleDestinyBuff:    1,
			DestinyReplenishCost: 1,
			ResumeEnabled:        1,
			HostModeEnabled:      1,
			HostMinPairs:         6,
			HostShareBps:         50,
			RevenueBase:          1,
			LedgerType:           0,
			Status:               1,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := db.Create(&rule).Error; err != nil {
			return err
		}
	}

	var deckCount int64
	if err := db.Model(&MemoryMatchDeckTemplateRow{}).Count(&deckCount).Error; err != nil {
		return err
	}
	if deckCount == 0 {
		now := time.Now()
		if err := db.Create(&MemoryMatchDeckTemplateRow{
			ID: 1, Name: "默认卡组", Status: 1, TotalCards: 54, Weight: 0,
			ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}
	}

	var appearanceCount int64
	if err := db.Model(&MemoryMatchAppearanceRow{}).Count(&appearanceCount).Error; err != nil {
		return err
	}
	if appearanceCount == 0 {
		now := time.Now()
		rows := []MemoryMatchAppearanceRow{
			{ID: 1, AppearanceType: 2, Name: "默认卡背", Status: 1, Free: 1, ExchangePoints: "0.00", CreatedAt: now, UpdatedAt: now},
		}
		for i := 1; i <= 9; i++ {
			rows = append(rows, MemoryMatchAppearanceRow{
				ID: int64(i + 1), AppearanceType: 1, Name: strconv.Itoa(i), Status: 1, Free: 1,
				ExchangePoints: "0.00", CreatedAt: now, UpdatedAt: now,
			})
		}
		if err := db.Create(&rows).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *MemoryMatchAdminStore) GetGlobalRules(ctx context.Context) (*MemoryMatchGlobalRuleRow, error) {
	var row MemoryMatchGlobalRuleRow
	if err := s.db.WithContext(ctx).Where("id = ?", 1).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *MemoryMatchAdminStore) SaveGlobalRules(ctx context.Context, row *MemoryMatchGlobalRuleRow) error {
	row.ID = 1
	row.UpdatedAt = time.Now()
	return s.db.WithContext(ctx).Save(row).Error
}

func (s *MemoryMatchAdminStore) ListDeckTemplates(ctx context.Context, keyword string, status int64, page, pageSize int) ([]MemoryMatchDeckTemplateRow, int64, error) {
	query := s.db.WithContext(ctx).Model(&MemoryMatchDeckTemplateRow{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []MemoryMatchDeckTemplateRow
	err := query.Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	return rows, total, err
}

func (s *MemoryMatchAdminStore) SaveDeckTemplate(ctx context.Context, row *MemoryMatchDeckTemplateRow) error {
	now := time.Now()
	row.UpdatedAt = now
	if row.ID == 0 {
		row.CreatedAt = now
		return s.db.WithContext(ctx).Create(row).Error
	}
	return s.db.WithContext(ctx).Save(row).Error
}

func (s *MemoryMatchAdminStore) DeleteDeckTemplate(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&MemoryMatchDeckTemplateRow{}, id).Error
}

func (s *MemoryMatchAdminStore) ListAppearances(ctx context.Context, appearanceType, status int64, keyword string, page, pageSize int) ([]MemoryMatchAppearanceRow, int64, error) {
	query := s.db.WithContext(ctx).Model(&MemoryMatchAppearanceRow{})
	if appearanceType >= 0 {
		query = query.Where("appearance_type = ?", appearanceType)
	}
	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []MemoryMatchAppearanceRow
	err := query.Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	return rows, total, err
}

func (s *MemoryMatchAdminStore) SaveAppearance(ctx context.Context, row *MemoryMatchAppearanceRow) error {
	now := time.Now()
	row.UpdatedAt = now
	if row.ID == 0 {
		row.CreatedAt = now
		return s.db.WithContext(ctx).Create(row).Error
	}
	return s.db.WithContext(ctx).Save(row).Error
}

func (s *MemoryMatchAdminStore) DeleteAppearance(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&MemoryMatchAppearanceRow{}, id).Error
}
