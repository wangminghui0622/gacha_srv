// Package repo 指标接入存储编排 + 腾讯广告点击存储。
// 语义照抄 server_v2 integration internal/data/mysql.go（MySQLRepository.Ingest）：
//   - findBatch：先 (tenant,request_id) 再 (tenant,provider_code,idempotency_key)，
//     matchBatch 比对 TenantID/RequestID/IdempotencyKey/ProviderCode/RequestHash
//     （不比 ExternalAccountID），异键同内容 → 重放，同键异内容 → ErrIdempotencyConflict；
//   - 插入批次遇唯一键冲突（并发同键）→ 回滚 → 重查重放；
//   - 逐样本 UpsertCurrent：affected==0 ⇒ stale（is_stale=1 落 raw 但不发事件）；
//   - aggregateID = sha256 hex of JSON [tenant,provider,account,object,metric,bucketStart.Unix(),seconds,dimension]；
//   - 非 stale 样本写 outbox 事件（payload EventId 回填 outbox 行 id）。
//
// 偏离记录（M15d D5）：server_v2 雪花 ID 已知，先 UpsertCurrent 再 InsertRaw；
// 本仓 raw id 由 AUTO_INCREMENT 生成，故先插 raw 取 id 再 upsert current——
// 同事务内顺序无语义差异。outbox 行 id 也是插入后才知道，payload 先以 EventId=0
// 写入、同一事务内回填真实事件 id（老项目 payload.EventId == outbox 行 id 不变式保持）。
package repo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/thirdparty/internal/core"
	"gacha_srv/app/thirdparty/internal/tencentad"
	"gacha_srv/app/thirdparty/thirdparty"
	model "gacha_srv/pkg/model_nocache/gacha_thirdparty"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// MetricSnapshotEventType 指标快照事件类型（server_v2 MetricSnapshotEventType 同款）。
const MetricSnapshotEventType = "integration.metric.snapshot.changed.v1"

// MetricStore 指标接入存储（server_v2 MySQLRepository 移植）。
type MetricStore struct {
	db    *gorm.DB
	batch model.IntegrationMetricBatchModel
}

// NewMetricStore 构建存储。
func NewMetricStore(db *gorm.DB, batch model.IntegrationMetricBatchModel) *MetricStore {
	return &MetricStore{db: db, batch: batch}
}

// Ingest 接入一批样本；返回批次与是否重放（server_v2 Ingest 同款签名语义）。
func (s *MetricStore) Ingest(ctx context.Context, b *core.Batch) (*core.Batch, bool, error) {
	existing, err := s.findBatch(ctx, b)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, true, nil
	}
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, false, fmt.Errorf("begin metric ingestion: %w", tx.Error)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	row := &model.IntegrationMetricBatch{
		TenantId:          b.TenantId,
		RequestId:         b.RequestId,
		IdempotencyKey:    b.IdempotencyKey,
		RequestHash:       string(b.RequestHash[:]),
		ProviderCode:      b.ProviderCode,
		ExternalAccountId: b.ExternalAccountId,
		CreatedAt:         now,
	}
	if err = tx.WithContext(ctx).Create(row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// 并发同键：回滚后重查（server_v2 InsertBatch 1062 → rollback → refind → replay）
			_ = tx.Rollback()
			replay, lookupErr := s.findBatch(ctx, b)
			if lookupErr != nil {
				return nil, false, lookupErr
			}
			if replay != nil {
				return replay, true, nil
			}
			return nil, false, fmt.Errorf("metric batch unique-key race: %w", err)
		}
		return nil, false, fmt.Errorf("insert metric batch: %w", err)
	}
	b.Id = row.Id

	for _, smp := range b.Samples {
		raw := &model.IntegrationMetricRaw{
			TenantId:           b.TenantId,
			BatchId:            b.Id,
			ProviderCode:       b.ProviderCode,
			ExternalAccountId:  b.ExternalAccountId,
			ExternalObjectId:   smp.ExternalObjectId,
			ProviderMetricCode: smp.ProviderMetricCode,
			StandardMetric:     uint64(smp.StandardMetric),
			BucketStart:        smp.BucketStart,
			BucketSeconds:      uint64(smp.BucketSeconds),
			DimensionKey:       smp.DimensionKey,
			Value:              smp.Value,
			Currency:           smp.Currency,
			SourceRevision:     smp.SourceRevision,
			CreatedAt:          now,
		}
		if err = tx.WithContext(ctx).Create(raw).Error; err != nil {
			return nil, false, fmt.Errorf("insert raw metric: %w", err)
		}
		affected, upsertErr := model.NewIntegrationMetricCurrentModel(tx).UpsertCurrent(ctx, nil, &model.IntegrationMetricCurrent{
			TenantId:          b.TenantId,
			ProviderCode:      b.ProviderCode,
			ExternalAccountId: b.ExternalAccountId,
			ExternalObjectId:  smp.ExternalObjectId,
			StandardMetric:    uint64(smp.StandardMetric),
			BucketStart:       smp.BucketStart,
			BucketSeconds:     uint64(smp.BucketSeconds),
			DimensionKey:      smp.DimensionKey,
			Value:             smp.Value,
			Currency:          smp.Currency,
			SourceRevision:    smp.SourceRevision,
			RawId:             raw.Id,
			UpdatedAt:         now,
		})
		if upsertErr != nil {
			return nil, false, fmt.Errorf("upsert normalized metric: %w", upsertErr)
		}
		stale := affected == 0
		if stale {
			b.StaleCount++
			if err = tx.WithContext(ctx).Model(&model.IntegrationMetricRaw{}).
				Where("`id` = ?", raw.Id).Update("is_stale", 1).Error; err != nil {
				return nil, false, fmt.Errorf("flag stale raw metric: %w", err)
			}
		} else {
			b.AcceptedCount++
		}
		if !stale {
			aggregateID, keyErr := metricAggregateID(b, smp)
			if keyErr != nil {
				return nil, false, fmt.Errorf("encode metric aggregate key: %w", keyErr)
			}
			event := &thirdparty.MetricSnapshotChanged{
				EventId:           0, // 占位：outbox 行 id 插入后回填
				TenantId:          int64(b.TenantId),
				RawId:             raw.Id,
				BatchId:           b.Id,
				ProviderCode:      b.ProviderCode,
				ExternalAccountId: b.ExternalAccountId,
				ExternalObjectId:  smp.ExternalObjectId,
				StandardMetric:    smp.StandardMetric,
				BucketStart:       timestamppb.New(smp.BucketStart),
				BucketSeconds:     smp.BucketSeconds,
				DimensionKey:      smp.DimensionKey,
				Value:             smp.Value,
				Currency:          smp.Currency,
				SourceRevision:    smp.SourceRevision,
				OccurredAt:        timestamppb.New(now),
			}
			payload, marshalErr := proto.Marshal(event)
			if marshalErr != nil {
				return nil, false, fmt.Errorf("marshal metric snapshot: %w", marshalErr)
			}
			om := &model.IntegrationMetricOutbox{
				AggregateId: aggregateID,
				EventType:   MetricSnapshotEventType,
				Payload:     string(payload),
				Status:      1,
				AvailableAt: now,
				CreatedAt:   now,
			}
			if err = tx.WithContext(ctx).Create(om).Error; err != nil {
				return nil, false, fmt.Errorf("insert metric outbox: %w", err)
			}
			event.EventId = om.Id
			payload, marshalErr = proto.Marshal(event)
			if marshalErr != nil {
				return nil, false, fmt.Errorf("marshal metric snapshot: %w", marshalErr)
			}
			if err = tx.WithContext(ctx).Model(&model.IntegrationMetricOutbox{}).
				Where("`id` = ?", om.Id).Update("payload", string(payload)).Error; err != nil {
				return nil, false, fmt.Errorf("backfill metric outbox payload: %w", err)
			}
		}
	}
	if err = tx.WithContext(ctx).Model(&model.IntegrationMetricBatch{}).Where("`id` = ?", b.Id).
		Updates(map[string]any{"accepted_count": b.AcceptedCount, "stale_count": b.StaleCount}).Error; err != nil {
		return nil, false, fmt.Errorf("update metric batch counts: %w", err)
	}
	if err = tx.Commit().Error; err != nil {
		return nil, false, fmt.Errorf("commit metric ingestion: %w", err)
	}
	return b, false, nil
}

// findBatch server_v2 findBatch 同款：双幂等键查询 + matchBatch。
func (s *MetricStore) findBatch(ctx context.Context, b *core.Batch) (*core.Batch, error) {
	byRequest, err := s.batch.FindOneByTenantIdRequestId(ctx, b.TenantId, b.RequestId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("lookup metric request: %w", err)
	}
	if err == nil {
		return matchBatch(byRequest, b)
	}
	byKey, err := s.batch.FindOneByTenantIdProviderCodeIdempotencyKey(ctx, b.TenantId, b.ProviderCode, b.IdempotencyKey)
	if errors.Is(err, model.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup metric idempotency key: %w", err)
	}
	return matchBatch(byKey, b)
}

// matchBatch server_v2 matchBatch 同款（不含 ExternalAccountID 比对）。
func matchBatch(x *model.IntegrationMetricBatch, b *core.Batch) (*core.Batch, error) {
	if x.TenantId != b.TenantId || x.RequestId != b.RequestId || x.IdempotencyKey != b.IdempotencyKey || x.ProviderCode != b.ProviderCode || !bytes.Equal([]byte(x.RequestHash), b.RequestHash[:]) {
		return nil, core.ErrIdempotencyConflict
	}
	return &core.Batch{
		Id:                x.Id,
		TenantId:          x.TenantId,
		RequestId:         x.RequestId,
		IdempotencyKey:    x.IdempotencyKey,
		ProviderCode:      x.ProviderCode,
		ExternalAccountId: x.ExternalAccountId,
		AcceptedCount:     x.AcceptedCount,
		StaleCount:        x.StaleCount,
	}, nil
}

// metricAggregateID server_v2 metricAggregateID 同款：槽位 JSON 的 sha256 hex。
func metricAggregateID(b *core.Batch, s core.Sample) (string, error) {
	key, err := json.Marshal([]any{b.TenantId, b.ProviderCode, b.ExternalAccountId, s.ExternalObjectId, s.StandardMetric, s.BucketStart.Unix(), s.BucketSeconds, s.DimensionKey})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:]), nil
}

// TxAdClickStore 腾讯广告点击存储（server_v2 data.NewTxAdClickStore 同款职责）。
type TxAdClickStore struct {
	model model.IntegrationTxAdClickModel
}

// NewTxAdClickStore 构建存储。
func NewTxAdClickStore(m model.IntegrationTxAdClickModel) *TxAdClickStore {
	return &TxAdClickStore{model: m}
}

// CountByOpenID 按 openid 计数（去重先行检查）。
func (s *TxAdClickStore) CountByOpenID(ctx context.Context, openID string) (int64, error) {
	return s.model.CountByWechatOpenId(ctx, openID)
}

// Insert 落库点击行（AUTO_INCREMENT 生成 id）。
func (s *TxAdClickStore) Insert(ctx context.Context, row *tencentad.AdClick) error {
	return s.model.Insert(ctx, nil, &model.IntegrationTxAdClick{
		TenantId:     row.TenantID,
		AccountId:    row.AccountID,
		ClickId:      row.ClickID,
		ClickTime:    row.ClickTime,
		WechatOpenId: row.WechatOpenID,
		DeviceOsType: row.DeviceOsType,
		CreatedAt:    time.Now().UTC(),
	})
}
