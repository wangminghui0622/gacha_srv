// Package core 指标接入纯函数（无 IO，可单测）：请求校验/批次构建。
// 语义照抄 server_v2 integration internal/biz/metrics.go：
//   - validateRequest：身份键 boundedCode、样本 1..200、标识字段长度上限、
//     bucket_start UTC 对齐（nanosecond==0 且 unix%seconds==0）、seconds∈{3600,86400}、
//     value>=0、计数指标(1-3)币种空/金额指标(4-5)币种 3 位大写、槽位去重、
//     source_revision==0 非法；
//   - request_hash = SHA256(deterministic proto marshal(req))，幂等比对用（不含批次 ID）。
//
// 偏离：server_v2 用雪花 ID（Batch/Sample.ID 插入前已知），本仓批次/样本/事件 ID
// 用 AUTO_INCREMENT（M15d D5），故 Sample/Batch 不含 ID 字段——repo 层插入后回填。
package core

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"gacha_srv/app/thirdparty/thirdparty"

	"google.golang.org/protobuf/proto"
)

var (
	// ErrInvalidRequest 请求校验失败（errorx 6011）
	ErrInvalidRequest = errors.New("invalid metric request")
	// ErrIdempotencyConflict 幂等键冲突：同 request_id/幂等键但请求内容不同（errorx 6012）
	ErrIdempotencyConflict = errors.New("metric ingestion idempotency conflict")
)

// Sample 归一化指标样本（无 ID，AUTO_INCREMENT 由 repo 回填）。
type Sample struct {
	ProviderMetricCode string
	StandardMetric     thirdparty.StandardMetric
	ExternalObjectId   string
	BucketStart        time.Time // UTC
	BucketSeconds      uint32
	DimensionKey       string
	Value              int64
	Currency           string
	SourceRevision     uint64
}

// Batch 指标接入批次（server_v2 biz.Batch 同款；ID 由 repo 插入后回填，计数在
// 样本循环内累计——重放结果直接来自批次行，重放批次 Samples 为空）。
type Batch struct {
	Id                uint64
	TenantId          uint64
	RequestId         string
	IdempotencyKey    string
	ProviderCode      string
	ExternalAccountId string
	RequestHash       [32]byte
	Samples           []Sample
	AcceptedCount     uint64
	StaleCount        uint64
}

// BuildBatch 校验请求并构建批次（server_v2 Integration.Ingest 同款，去掉 ID 生成）。
func BuildBatch(r *thirdparty.IngestProviderMetricsReq) (*Batch, error) {
	if err := validateRequest(r); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("marshal metric request: %w", err)
	}
	b := &Batch{
		TenantId:          uint64(r.TenantId),
		RequestId:         r.RequestId,
		IdempotencyKey:    r.IdempotencyKey,
		ProviderCode:      r.ProviderCode,
		ExternalAccountId: r.ExternalAccountId,
		RequestHash:       sha256.Sum256(wire),
	}
	for _, s := range r.Samples {
		b.Samples = append(b.Samples, Sample{
			ProviderMetricCode: s.ProviderMetricCode,
			StandardMetric:     s.StandardMetric,
			ExternalObjectId:   s.ExternalObjectId,
			BucketStart:        s.BucketStart.AsTime().UTC(),
			BucketSeconds:      s.BucketSeconds,
			DimensionKey:       s.DimensionKey,
			Value:              s.Value,
			Currency:           s.Currency,
			SourceRevision:     s.SourceRevision,
		})
	}
	return b, nil
}

// validateRequest server_v2 biz.validateRequest 逐条同款（错误文案保持一致）。
func validateRequest(r *thirdparty.IngestProviderMetricsReq) error {
	if r == nil {
		return errors.New("metric request is required")
	}
	if r.TenantId == 0 || !boundedCode(r.RequestId, 64) || !boundedCode(r.IdempotencyKey, 128) || !boundedCode(r.ProviderCode, 64) || !boundedCode(r.ExternalAccountId, 128) {
		return errors.New("metric request identity is invalid")
	}
	if len(r.Samples) == 0 || len(r.Samples) > 200 {
		return errors.New("metric sample count must be 1..200")
	}
	type slot struct {
		object    string
		metric    thirdparty.StandardMetric
		start     int64
		seconds   uint32
		dimension string
	}
	seen := make(map[slot]bool, len(r.Samples))
	for i, s := range r.Samples {
		if s == nil || !boundedCode(s.ProviderMetricCode, 128) || !boundedCode(s.ExternalObjectId, 128) || !boundedCode(s.DimensionKey, 256) {
			return fmt.Errorf("metric sample %d identifier is invalid", i)
		}
		if s.BucketStart == nil || s.BucketStart.CheckValid() != nil || s.SourceRevision == 0 {
			return fmt.Errorf("metric sample %d timestamp or revision is invalid", i)
		}
		if s.BucketSeconds != 3600 && s.BucketSeconds != 86400 {
			return fmt.Errorf("metric sample %d bucket size is invalid", i)
		}
		at := s.BucketStart.AsTime().UTC()
		if at.Nanosecond() != 0 || at.Unix()%int64(s.BucketSeconds) != 0 {
			return fmt.Errorf("metric sample %d bucket start is not UTC-aligned", i)
		}
		if s.Value < 0 {
			return fmt.Errorf("metric sample %d value is negative", i)
		}
		switch s.StandardMetric {
		case thirdparty.StandardMetric_STANDARD_METRIC_IMPRESSIONS, thirdparty.StandardMetric_STANDARD_METRIC_CLICKS, thirdparty.StandardMetric_STANDARD_METRIC_CONVERSIONS:
			if s.Currency != "" {
				return fmt.Errorf("metric sample %d count currency must be empty", i)
			}
		case thirdparty.StandardMetric_STANDARD_METRIC_SPEND, thirdparty.StandardMetric_STANDARD_METRIC_REVENUE:
			if !currencyCode(s.Currency) {
				return fmt.Errorf("metric sample %d currency is invalid", i)
			}
		default:
			return fmt.Errorf("metric sample %d standard metric is invalid", i)
		}
		key := slot{s.ExternalObjectId, s.StandardMetric, at.Unix(), s.BucketSeconds, s.DimensionKey}
		if seen[key] {
			return fmt.Errorf("metric sample %d repeats a normalized slot", i)
		}
		seen[key] = true
	}
	return nil
}

// boundedCode server_v2 同款：非空、≤max、无首尾空白、无控制字符。
func boundedCode(s string, max int) bool {
	return len(s) > 0 && len(s) <= max && s == strings.TrimSpace(s) && !strings.ContainsAny(s, "\x00\n\r\t")
}

// currencyCode 三字母大写 ISO 币种（server_v2 同款）。
func currencyCode(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
