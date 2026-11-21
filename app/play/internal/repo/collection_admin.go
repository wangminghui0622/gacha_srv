package repo

import (
	"time"

	"gorm.io/gorm"
)

// CollectionSetRow 图鉴集合（管理端）。每个版本一行，code 唯一。
type CollectionSetRow struct {
	ID             int64     `gorm:"column:id;primaryKey" json:"id"`
	Code           string    `gorm:"column:code;size:64;not null;uniqueIndex" json:"code"`
	Name           string    `gorm:"column:name;size:128;not null" json:"name"`
	Version        int64     `gorm:"column:version;not null;default:1" json:"version"`
	Status         int64     `gorm:"column:status;not null;default:1" json:"status"`
	AccumulateType int64     `gorm:"column:accumulate_type;not null;default:1" json:"accumulateType"`
	CycleType      int64     `gorm:"column:cycle_type;not null;default:0" json:"cycleType"`
	Sort           int64     `gorm:"column:sort;not null;default:0" json:"sort"`
	ConfigJSON     string    `gorm:"column:config_json;type:json" json:"configJson"`
	StartAt        string    `gorm:"column:start_at;size:32;not null;default:''" json:"startAt"`
	EndAt          string    `gorm:"column:end_at;size:32;not null;default:''" json:"endAt"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (CollectionSetRow) TableName() string { return "collection_sets" }

// EnsureCollectionAdminSeed 建表并写入截图中的示例集合（仅空表时）。
func EnsureCollectionAdminSeed(db *gorm.DB) error {
	if err := db.AutoMigrate(&CollectionSetRow{}); err != nil {
		return err
	}
	var count int64
	if err := db.Model(&CollectionSetRow{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := time.Now()
	rows := []CollectionSetRow{
		{ID: 1, Code: "TEST_COLLECTION_POKEMON", Name: "宝可梦伙伴集结", Version: 1, Status: 3, AccumulateType: 1, CycleType: -1, Sort: 400, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
		{ID: 2, Code: "TEST_COLLECTION_ONE_PIECE", Name: "伟大航路伙伴图鉴", Version: 1, Status: 2, AccumulateType: 1, CycleType: 3, Sort: 300, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
		{ID: 3, Code: "TEST_COLLECTION_GUNDAM", Name: "高达机甲集结", Version: 1, Status: 2, AccumulateType: 2, CycleType: -1, Sort: 200, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
		{ID: 4, Code: "TEST_COLLECTION_CHIKAWA", Name: "吉伊卡哇冬日伙伴", Version: 1, Status: 2, AccumulateType: 1, CycleType: 0, Sort: 100, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
		{ID: 5, Code: "COL_20260922093645_DA274806F5EC", Name: "【凡人修仙图鉴】", Version: 1, Status: 2, AccumulateType: 2, CycleType: 0, Sort: 10, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
		{ID: 6, Code: "COL_20260922100339_09AF547DBF38", Name: "【凡人修仙图鉴】", Version: 2, Status: 1, AccumulateType: 2, CycleType: 0, Sort: 10, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
		{ID: 7, Code: "COL_20260923030234_4B4E061DF1CF", Name: "【凡人修仙图鉴】", Version: 3, Status: 2, AccumulateType: 2, CycleType: 0, Sort: 10, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
		{ID: 8, Code: "COL_20260923040011_4E4ECA2C28F1", Name: "【凡人修仙图鉴】", Version: 4, Status: 1, AccumulateType: 2, CycleType: 0, Sort: 10, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
		{ID: 9, Code: "COL_20261009073326_25A8FDD8EE9F", Name: "【凡人修仙图鉴】", Version: 5, Status: 1, AccumulateType: 2, CycleType: 0, Sort: 10, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
	}
	return db.Create(&rows).Error
}

type CollectionAdminStore struct {
	db *gorm.DB
}

func NewCollectionAdminStore(db *gorm.DB) *CollectionAdminStore {
	return &CollectionAdminStore{db: db}
}

func (s *CollectionAdminStore) ListCollectionSets(keyword string, status, page, pageSize int64) ([]CollectionSetRow, int64, error) {
	query := s.db.Model(&CollectionSetRow{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("code LIKE ? OR name LIKE ?", like, like)
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []CollectionSetRow
	if err := query.Order("sort DESC, id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
