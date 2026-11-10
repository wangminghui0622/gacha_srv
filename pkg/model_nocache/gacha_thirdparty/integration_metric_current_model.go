// 手写模型：integration_metric_current 为联合主键（8 列），goctl model 不支持，
// 无法自动生成——风格对齐 goctl 生成物。
// UpsertCurrent 语义照抄 server_v2 UpsertCurrent：新修订号严格大于才更新
// （IF(VALUES(source_revision)>source_revision)），affected==0 ⇒ stale（clientFoundRows=false）。
package gacha_thirdparty

import (
	"context"
	"time"

	"github.com/SpectatorNan/gorm-zero/gormc"
	"gorm.io/gorm"
)

type (
	// IntegrationMetricCurrentModel 指标当前值模型（槽位唯一，修订号仲裁）。
	IntegrationMetricCurrentModel interface {
		FindOne(ctx context.Context, tx *gorm.DB, data *IntegrationMetricCurrent) (*IntegrationMetricCurrent, error)
		UpsertCurrent(ctx context.Context, tx *gorm.DB, data *IntegrationMetricCurrent) (int64, error)
	}

	defaultIntegrationMetricCurrentModel struct {
		conn  *gorm.DB
		table string
	}

	IntegrationMetricCurrent struct {
		TenantId          uint64    `gorm:"column:tenant_id;primary_key"`           // 租户ID
		ProviderCode      string    `gorm:"column:provider_code;primary_key"`       // 渠道编码
		ExternalAccountId string    `gorm:"column:external_account_id;primary_key"` // 外部账户ID
		ExternalObjectId  string    `gorm:"column:external_object_id;primary_key"`  // 外部对象ID
		StandardMetric    uint64    `gorm:"column:standard_metric;primary_key"`     // 标准指标 1-5
		BucketStart       time.Time `gorm:"column:bucket_start;primary_key"`        // 时间桶起点（UTC 对齐）
		BucketSeconds     uint64    `gorm:"column:bucket_seconds;primary_key"`      // 桶长秒（3600/86400）
		DimensionKey      string    `gorm:"column:dimension_key;primary_key"`       // 维度键
		Value             int64     `gorm:"column:value"`                           // 指标值
		Currency          string    `gorm:"column:currency"`                        // 币种（计数指标空）
		SourceRevision    uint64    `gorm:"column:source_revision"`                 // 源修订号
		RawId             uint64    `gorm:"column:raw_id"`                          // 最新接受的样本ID
		UpdatedAt         time.Time `gorm:"column:updated_at"`
	}
)

func (IntegrationMetricCurrent) TableName() string {
	return "`integration_metric_current`"
}

// NewIntegrationMetricCurrentModel 返回模型（手写，对齐 goctl 风格）。
func NewIntegrationMetricCurrentModel(db *gorm.DB) IntegrationMetricCurrentModel {
	return &defaultIntegrationMetricCurrentModel{conn: db, table: "`integration_metric_current`"}
}

func (m *defaultIntegrationMetricCurrentModel) FindOne(ctx context.Context, tx *gorm.DB, data *IntegrationMetricCurrent) (*IntegrationMetricCurrent, error) {
	db := m.conn
	if tx != nil {
		db = tx
	}
	var resp IntegrationMetricCurrent
	err := db.WithContext(ctx).Model(&IntegrationMetricCurrent{}).
		Where("`tenant_id` = ? AND `provider_code` = ? AND `external_account_id` = ? AND `external_object_id` = ? AND `standard_metric` = ? AND `bucket_start` = ? AND `bucket_seconds` = ? AND `dimension_key` = ?",
			data.TenantId, data.ProviderCode, data.ExternalAccountId, data.ExternalObjectId,
			data.StandardMetric, data.BucketStart, data.BucketSeconds, data.DimensionKey).
		Take(&resp).Error
	switch err {
	case nil:
		return &resp, nil
	case gormc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// UpsertCurrent 条件 upsert：仅当新 source_revision 严格大于存量才更新值列；
// source_revision 本身取 GREATEST。返回 RowsAffected：0 ⇒ stale（旧/同修订）。
// SQL 与 server_v2 UpsertCurrent 逐字同款。
func (m *defaultIntegrationMetricCurrentModel) UpsertCurrent(ctx context.Context, tx *gorm.DB, data *IntegrationMetricCurrent) (int64, error) {
	db := m.conn
	if tx != nil {
		db = tx
	}
	res := db.WithContext(ctx).Exec(
		"INSERT INTO `integration_metric_current` "+
			"(`tenant_id`,`provider_code`,`external_account_id`,`external_object_id`,`standard_metric`,`bucket_start`,`bucket_seconds`,`dimension_key`,`value`,`currency`,`source_revision`,`raw_id`,`updated_at`) "+
			"VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) "+
			"ON DUPLICATE KEY UPDATE "+
			"`value` = IF(VALUES(`source_revision`) > `source_revision`,VALUES(`value`),`value`), "+
			"`currency` = IF(VALUES(`source_revision`) > `source_revision`,VALUES(`currency`),`currency`), "+
			"`raw_id` = IF(VALUES(`source_revision`) > `source_revision`,VALUES(`raw_id`),`raw_id`), "+
			"`updated_at` = IF(VALUES(`source_revision`) > `source_revision`,VALUES(`updated_at`),`updated_at`), "+
			"`source_revision` = GREATEST(`source_revision`,VALUES(`source_revision`))",
		data.TenantId, data.ProviderCode, data.ExternalAccountId, data.ExternalObjectId,
		data.StandardMetric, data.BucketStart, data.BucketSeconds, data.DimensionKey,
		data.Value, data.Currency, data.SourceRevision, data.RawId, data.UpdatedAt)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
