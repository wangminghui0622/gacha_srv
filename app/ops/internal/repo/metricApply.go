// 指标事件应用（M15d）：integration 指标管线落 ops 侧（inbox 去重 + 快照修订守卫 + 桶差量）。
// 语义照抄 server_v2 reporting internal/data/mysql.go ApplyMetric：
//   - reporting_metric_inbox INSERT IGNORE（event_id uk 去重），0 行 ⇒ 已处理过 applied=false；
//   - reporting_metric_snapshot 8 键 FOR UPDATE 行锁；source_revision<=旧值 ⇒ 过期不应用
//     （inbox 行仍提交——记录已收到过该事件）；
//   - biz.Deltas(old,new,oldCur,newCur) → 按币种桶 AddDelta（ON DUPLICATE value=value+delta）；
//   - snapshot 全量 upsert（含 event_id 最新应用事件）。
// 同事务提交；任何失败整体回滚（幂等安全，dispatcher 可重试）。
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// MetricSnapshotEventType 指标快照事件类型（server_v2 MetricSnapshotEventType 同款）。
const MetricSnapshotEventType = "integration.metric.snapshot.changed.v1"

// MetricEvent 待应用事件（server_v2 biz.MetricEvent 同款形状）。
type MetricEvent struct {
	EventID, TenantID                                                         uint64
	ProviderCode, ExternalAccountID, ExternalObjectID, DimensionKey, Currency string
	StandardMetric                                                            ops.StandardMetric
	BucketStart                                                               time.Time
	BucketSeconds                                                             uint32
	Value                                                                     int64
	SourceRevision                                                            uint64
}

// ApplyMetric 应用一条指标事件（server_v2 MySQLRepository.ApplyMetric 逐条同款）。
func (s *ReportingStore) ApplyMetric(ctx context.Context, e *MetricEvent) (bool, error) {
	var applied bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		inserted, err := opsmodel.NewReportingMetricInboxModel(tx).InsertIgnore(ctx, nil, &opsmodel.ReportingMetricInbox{
			EventId: e.EventID, TenantId: e.TenantID, EventType: MetricSnapshotEventType,
			SourceRevision: e.SourceRevision, ReceivedAt: now,
		})
		if err != nil {
			return fmt.Errorf("insert metric inbox: %w", err)
		}
		if inserted == 0 {
			return nil
		}
		key := &opsmodel.ReportingMetricSnapshot{
			TenantId: e.TenantID, ProviderCode: e.ProviderCode, ExternalAccountId: e.ExternalAccountID,
			ExternalObjectId: e.ExternalObjectID, StandardMetric: uint64(e.StandardMetric),
			BucketStart: e.BucketStart.UTC(), BucketSeconds: uint64(e.BucketSeconds), DimensionKey: e.DimensionKey,
		}
		snapshotModel := opsmodel.NewReportingMetricSnapshotModel(tx)
		old, err := snapshotModel.FindOneForUpdate(ctx, nil, key)
		if err != nil && !errors.Is(err, opsmodel.ErrNotFound) {
			return fmt.Errorf("lock metric snapshot: %w", err)
		}
		if err == nil && e.SourceRevision <= old.SourceRevision {
			return nil
		}
		oldValue, oldCurrency := int64(0), ""
		if err == nil {
			oldValue, oldCurrency = old.Value, old.Currency
		}
		bucketModel := opsmodel.NewReportingMetricBucketModel(tx)
		for currency, delta := range core.Deltas(oldValue, e.Value, oldCurrency, e.Currency) {
			if err = bucketModel.AddDelta(ctx, nil, &opsmodel.ReportingMetricBucket{
				TenantId: e.TenantID, ProviderCode: e.ProviderCode, ExternalAccountId: e.ExternalAccountID,
				StandardMetric: uint64(e.StandardMetric), BucketSeconds: uint64(e.BucketSeconds),
				BucketStart: e.BucketStart.UTC(), DimensionKey: e.DimensionKey,
				Currency: currency, Value: delta, UpdatedAt: now,
			}); err != nil {
				return fmt.Errorf("apply metric bucket delta: %w", err)
			}
		}
		key.Value, key.Currency = e.Value, e.Currency
		key.SourceRevision, key.EventId, key.UpdatedAt = e.SourceRevision, e.EventID, now
		if err = snapshotModel.Upsert(ctx, nil, key); err != nil {
			return fmt.Errorf("upsert metric snapshot: %w", err)
		}
		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

// BucketQuery 桶范围查询（server_v2 biz.Query 的子集：看板 widget 查询用）。
type BucketQuery struct {
	TenantID                                      uint64
	ProviderCode, ExternalAccountID, DimensionKey string
	StandardMetric                                uint64
	BucketSeconds                                 uint64
	From, To                                      time.Time
	Limit                                         int32
}

// ListBuckets 指标桶查询（server_v2 ListMetricBuckets 同款：
// 8 键等值 + bucket_start∈[from,to) + ORDER BY bucket_start ASC LIMIT）。
func (s *ReportingStore) ListBuckets(ctx context.Context, q BucketQuery) ([]*ops.WidgetDataPoint, error) {
	rows, err := opsmodel.NewReportingMetricBucketModel(s.db).ListRange(ctx,
		q.TenantID, q.ProviderCode, q.ExternalAccountID, q.StandardMetric, q.BucketSeconds,
		q.DimensionKey, q.From.UTC(), q.To.UTC(), int(q.Limit))
	if err != nil {
		return nil, err
	}
	out := make([]*ops.WidgetDataPoint, 0, len(rows))
	for i := range rows {
		x := &rows[i]
		out = append(out, &ops.WidgetDataPoint{
			BucketStart: timestamppb.New(x.BucketStart), Currency: x.Currency, Value: x.Value,
		})
	}
	return out, nil
}
