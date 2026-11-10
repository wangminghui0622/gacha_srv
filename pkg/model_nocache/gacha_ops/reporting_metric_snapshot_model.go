// 手写模型：reporting_metric_snapshot 为联合主键（8 列），goctl model 不支持——
// 风格对齐 goctl 生成物。语义照抄 server_v2 reporting：
//   - FindOneForUpdate：SELECT ... FOR UPDATE（ApplyMetric 行锁串行化）
//   - Upsert：ON DUPLICATE KEY UPDATE 全值覆盖（新修订已过守卫）
package gacha_ops

import (
	"context"
	"time"

	"github.com/SpectatorNan/gorm-zero/gormc"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type (
	// ReportingMetricSnapshotModel 指标快照模型（槽位唯一，修订号守卫）。
	ReportingMetricSnapshotModel interface {
		FindOneForUpdate(ctx context.Context, tx *gorm.DB, data *ReportingMetricSnapshot) (*ReportingMetricSnapshot, error)
		Upsert(ctx context.Context, tx *gorm.DB, data *ReportingMetricSnapshot) error
	}

	defaultReportingMetricSnapshotModel struct {
		conn  *gorm.DB
		table string
	}

	ReportingMetricSnapshot struct {
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
		SourceRevision    uint64    `gorm:"column:source_revision"`                 // 源修订号（严格大于才应用）
		EventId           uint64    `gorm:"column:event_id"`                        // 最新应用的事件ID
		UpdatedAt         time.Time `gorm:"column:updated_at"`
	}
)

func (ReportingMetricSnapshot) TableName() string {
	return "`reporting_metric_snapshot`"
}

// NewReportingMetricSnapshotModel 返回模型（手写，对齐 goctl 风格）。
func NewReportingMetricSnapshotModel(db *gorm.DB) ReportingMetricSnapshotModel {
	return &defaultReportingMetricSnapshotModel{conn: db, table: "`reporting_metric_snapshot`"}
}

// FindOneForUpdate 行锁读取（server_v2 GetMetricSnapshotForUpdate 同款，无行报 ErrNotFound）。
func (m *defaultReportingMetricSnapshotModel) FindOneForUpdate(ctx context.Context, tx *gorm.DB, data *ReportingMetricSnapshot) (*ReportingMetricSnapshot, error) {
	db := m.conn
	if tx != nil {
		db = tx
	}
	var resp ReportingMetricSnapshot
	err := db.WithContext(ctx).Model(&ReportingMetricSnapshot{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
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

// Upsert 快照覆盖（server_v2 UpsertMetricSnapshot 逐字同款）。
func (m *defaultReportingMetricSnapshotModel) Upsert(ctx context.Context, tx *gorm.DB, data *ReportingMetricSnapshot) error {
	db := m.conn
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Exec(
		"INSERT INTO `reporting_metric_snapshot` "+
			"(`tenant_id`,`provider_code`,`external_account_id`,`external_object_id`,`standard_metric`,`bucket_start`,`bucket_seconds`,`dimension_key`,`value`,`currency`,`source_revision`,`event_id`,`updated_at`) "+
			"VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) "+
			"ON DUPLICATE KEY UPDATE `value`=VALUES(`value`),`currency`=VALUES(`currency`),`source_revision`=VALUES(`source_revision`),`event_id`=VALUES(`event_id`),`updated_at`=VALUES(`updated_at`)",
		data.TenantId, data.ProviderCode, data.ExternalAccountId, data.ExternalObjectId,
		data.StandardMetric, data.BucketStart, data.BucketSeconds, data.DimensionKey,
		data.Value, data.Currency, data.SourceRevision, data.EventId, data.UpdatedAt).Error
}
