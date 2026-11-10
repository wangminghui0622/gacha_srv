// 手写模型：reporting_metric_bucket 为联合主键（8 列，含 currency），goctl model 不支持——
// 风格对齐 goctl 生成物。语义照抄 server_v2 reporting：
//   - AddDelta：ON DUPLICATE KEY UPDATE value=value+VALUES(value)（货币差量聚合）
//   - ListRange：看板查询数据源（ListMetricBuckets 同款 WHERE/ORDER/LIMIT）
package gacha_ops

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type (
	// ReportingMetricBucketModel 指标桶聚合模型（看板查询数据源）。
	ReportingMetricBucketModel interface {
		AddDelta(ctx context.Context, tx *gorm.DB, data *ReportingMetricBucket) error
		ListRange(ctx context.Context, tenantId uint64, providerCode string, externalAccountId string,
			standardMetric uint64, bucketSeconds uint64, dimensionKey string, from time.Time, to time.Time, limit int) ([]ReportingMetricBucket, error)
	}

	defaultReportingMetricBucketModel struct {
		conn  *gorm.DB
		table string
	}

	ReportingMetricBucket struct {
		TenantId          uint64    `gorm:"column:tenant_id;primary_key"`           // 租户ID
		ProviderCode      string    `gorm:"column:provider_code;primary_key"`       // 渠道编码
		ExternalAccountId string    `gorm:"column:external_account_id;primary_key"` // 外部账户ID
		StandardMetric    uint64    `gorm:"column:standard_metric;primary_key"`     // 标准指标 1-5
		BucketSeconds     uint64    `gorm:"column:bucket_seconds;primary_key"`      // 桶长秒（3600/86400）
		BucketStart       time.Time `gorm:"column:bucket_start;primary_key"`        // 时间桶起点（UTC 对齐）
		DimensionKey      string    `gorm:"column:dimension_key;primary_key"`       // 维度键
		Currency          string    `gorm:"column:currency;primary_key"`            // 币种（计数指标空串）
		Value             int64     `gorm:"column:value"`                           // 聚合值
		UpdatedAt         time.Time `gorm:"column:updated_at"`
	}
)

func (ReportingMetricBucket) TableName() string {
	return "`reporting_metric_bucket`"
}

// NewReportingMetricBucketModel 返回模型（手写，对齐 goctl 风格）。
func NewReportingMetricBucketModel(db *gorm.DB) ReportingMetricBucketModel {
	return &defaultReportingMetricBucketModel{conn: db, table: "`reporting_metric_bucket`"}
}

// AddDelta 桶差量聚合（server_v2 AddMetricBucketDelta 逐字同款）。
func (m *defaultReportingMetricBucketModel) AddDelta(ctx context.Context, tx *gorm.DB, data *ReportingMetricBucket) error {
	db := m.conn
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Exec(
		"INSERT INTO `reporting_metric_bucket` "+
			"(`tenant_id`,`provider_code`,`external_account_id`,`standard_metric`,`bucket_seconds`,`bucket_start`,`dimension_key`,`currency`,`value`,`updated_at`) "+
			"VALUES (?,?,?,?,?,?,?,?,?,?) "+
			"ON DUPLICATE KEY UPDATE `value`=`value`+VALUES(`value`),`updated_at`=VALUES(`updated_at`)",
		data.TenantId, data.ProviderCode, data.ExternalAccountId, data.StandardMetric,
		data.BucketSeconds, data.BucketStart, data.DimensionKey, data.Currency, data.Value, data.UpdatedAt).Error
}

// ListRange 时间范围查询（server_v2 ListMetricBuckets 同款：[from,to) 升序限行）。
func (m *defaultReportingMetricBucketModel) ListRange(ctx context.Context, tenantId uint64, providerCode string, externalAccountId string,
	standardMetric uint64, bucketSeconds uint64, dimensionKey string, from time.Time, to time.Time, limit int) ([]ReportingMetricBucket, error) {
	var rows []ReportingMetricBucket
	err := m.conn.WithContext(ctx).Model(&ReportingMetricBucket{}).
		Where("`tenant_id` = ? AND `provider_code` = ? AND `external_account_id` = ? AND `standard_metric` = ? AND `bucket_seconds` = ? AND `dimension_key` = ? AND `bucket_start` >= ? AND `bucket_start` < ?",
			tenantId, providerCode, externalAccountId, standardMetric, bucketSeconds, dimensionKey, from, to).
		Order("`bucket_start` ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
