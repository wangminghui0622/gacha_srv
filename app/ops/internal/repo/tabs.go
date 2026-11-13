package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// TabItem 控制台页签（JSON 持久化的最小字段：路径 + 标题 i18n key）。
type TabItem struct {
	Path     string `json:"path"`
	TitleKey string `json:"titleKey"`
}

// TabRecord 某控制台账号的页签状态。
type TabRecord struct {
	Tabs       []TabItem
	ActivePath string
}

type consoleUserTabsRow struct {
	TenantId   uint64    `gorm:"column:tenant_id;primaryKey"`
	AccountId  uint64    `gorm:"column:account_id;primaryKey"`
	TabsJson   string    `gorm:"column:tabs_json"`
	ActivePath string    `gorm:"column:active_path"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (consoleUserTabsRow) TableName() string {
	return "console_user_tabs"
}

// TabStore 按 tenant_id + account_id 持久化控制台页签。
type TabStore struct {
	db *gorm.DB
}

func NewTabStore(db *gorm.DB) *TabStore {
	return &TabStore{db: db}
}

// Get 读取页签；账号从未保存过时返回 (nil, nil)。
func (s *TabStore) Get(ctx context.Context, tenantID, accountID uint64) (*TabRecord, error) {
	var row consoleUserTabsRow
	err := s.db.WithContext(ctx).
		Where("`tenant_id` = ? AND `account_id` = ?", tenantID, accountID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record := &TabRecord{ActivePath: row.ActivePath, Tabs: []TabItem{}}
	if row.TabsJson != "" {
		if err = json.Unmarshal([]byte(row.TabsJson), &record.Tabs); err != nil {
			return nil, fmt.Errorf("decode console_user_tabs: %w", err)
		}
	}
	if record.Tabs == nil {
		record.Tabs = []TabItem{}
	}
	return record, nil
}

// Put 写入或覆盖页签状态。
func (s *TabStore) Put(ctx context.Context, tenantID, accountID uint64, record *TabRecord) error {
	if record == nil {
		record = &TabRecord{}
	}
	tabs := record.Tabs
	if tabs == nil {
		tabs = []TabItem{}
	}
	blob, err := json.Marshal(tabs)
	if err != nil {
		return fmt.Errorf("encode console_user_tabs: %w", err)
	}
	return s.db.WithContext(ctx).Exec(
		"INSERT INTO `console_user_tabs` (`tenant_id`,`account_id`,`tabs_json`,`active_path`,`created_at`,`updated_at`) "+
			"VALUES (?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP) "+
			"ON DUPLICATE KEY UPDATE `tabs_json`=VALUES(`tabs_json`), `active_path`=VALUES(`active_path`), `updated_at`=CURRENT_TIMESTAMP",
		tenantID, accountID, string(blob), record.ActivePath,
	).Error
}
