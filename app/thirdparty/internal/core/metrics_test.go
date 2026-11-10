package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gacha_srv/app/thirdparty/thirdparty"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// validReq 合法请求基线（server_v2 校验边界单测同款思路）。
func validReq() *thirdparty.IngestProviderMetricsReq {
	return &thirdparty.IngestProviderMetricsReq{
		TenantId: 1, RequestId: "req-1", IdempotencyKey: "idem-1",
		ProviderCode: "ad", ExternalAccountId: "acc-1",
		Samples: []*thirdparty.ProviderMetricSample{validSample()},
	}
}

func validSample() *thirdparty.ProviderMetricSample {
	return &thirdparty.ProviderMetricSample{
		ProviderMetricCode: "impressions",
		StandardMetric:     thirdparty.StandardMetric_STANDARD_METRIC_IMPRESSIONS,
		ExternalObjectId:   "camp-1",
		BucketStart:        timestamppb.New(time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)),
		BucketSeconds:      3600,
		DimensionKey:       "all",
		Value:              100,
		SourceRevision:     1,
	}
}

func TestBuildBatchValid(t *testing.T) {
	b, err := BuildBatch(validReq())
	if err != nil {
		t.Fatalf("合法请求应通过: %v", err)
	}
	if b.TenantId != 1 || b.RequestId != "req-1" || b.IdempotencyKey != "idem-1" ||
		b.ProviderCode != "ad" || b.ExternalAccountId != "acc-1" || len(b.Samples) != 1 {
		t.Fatalf("批次字段丢失: %+v", b)
	}
	if b.RequestHash == [32]byte{} {
		t.Fatal("RequestHash 不能为零值")
	}
	s := b.Samples[0]
	if s.Value != 100 || s.BucketSeconds != 3600 || s.SourceRevision != 1 ||
		!s.BucketStart.Equal(time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("样本归一化错误: %+v", s)
	}
	// 确定性：同请求两次哈希一致；内容变化 → 哈希变化
	again, _ := BuildBatch(validReq())
	if again.RequestHash != b.RequestHash {
		t.Fatal("同请求哈希应确定")
	}
	other := validReq()
	other.Samples[0].Value = 101
	ob, _ := BuildBatch(other)
	if ob.RequestHash == b.RequestHash {
		t.Fatal("值变化应改变哈希")
	}
}

func TestBuildBatchIdentityValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*thirdparty.IngestProviderMetricsReq)
	}{
		{"tenant=0", func(r *thirdparty.IngestProviderMetricsReq) { r.TenantId = 0 }},
		{"request_id 空", func(r *thirdparty.IngestProviderMetricsReq) { r.RequestId = "" }},
		{"request_id 超长", func(r *thirdparty.IngestProviderMetricsReq) { r.RequestId = strings.Repeat("a", 65) }},
		{"request_id 首尾空白", func(r *thirdparty.IngestProviderMetricsReq) { r.RequestId = " req-1 " }},
		{"request_id 控制字符", func(r *thirdparty.IngestProviderMetricsReq) { r.RequestId = "req\n1" }},
		{"idempotency_key 超长", func(r *thirdparty.IngestProviderMetricsReq) { r.IdempotencyKey = strings.Repeat("a", 129) }},
		{"provider 空", func(r *thirdparty.IngestProviderMetricsReq) { r.ProviderCode = "" }},
		{"account 超长", func(r *thirdparty.IngestProviderMetricsReq) { r.ExternalAccountId = strings.Repeat("a", 129) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validReq()
			tc.mutate(r)
			if _, err := BuildBatch(r); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("期望 ErrInvalidRequest，得到 %v", err)
			}
		})
	}
}

func TestBuildBatchSampleValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*thirdparty.ProviderMetricSample)
	}{
		{"nil 样本", func(s *thirdparty.ProviderMetricSample) {}},
		{"metric_code 超长", func(s *thirdparty.ProviderMetricSample) { s.ProviderMetricCode = strings.Repeat("a", 129) }},
		{"object_id 超长", func(s *thirdparty.ProviderMetricSample) { s.ExternalObjectId = strings.Repeat("a", 129) }},
		{"dimension 超长", func(s *thirdparty.ProviderMetricSample) { s.DimensionKey = strings.Repeat("a", 257) }},
		{"dimension 控制字符", func(s *thirdparty.ProviderMetricSample) { s.DimensionKey = "a\tb" }},
		{"bucket_start 缺失", func(s *thirdparty.ProviderMetricSample) { s.BucketStart = nil }},
		{"source_revision=0", func(s *thirdparty.ProviderMetricSample) { s.SourceRevision = 0 }},
		{"bucket_seconds 非法", func(s *thirdparty.ProviderMetricSample) { s.BucketSeconds = 1800 }},
		{"纳秒未清零", func(s *thirdparty.ProviderMetricSample) {
			s.BucketStart = timestamppb.New(time.Date(2026, 10, 23, 8, 0, 0, 1, time.UTC))
		}},
		{"桶起点未对齐", func(s *thirdparty.ProviderMetricSample) {
			s.BucketStart = timestamppb.New(time.Date(2026, 10, 23, 8, 30, 0, 0, time.UTC))
		}},
		{"值为负", func(s *thirdparty.ProviderMetricSample) { s.Value = -1 }},
		{"计数指标带币种", func(s *thirdparty.ProviderMetricSample) { s.Currency = "CNY" }},
		{"标准指标未指定", func(s *thirdparty.ProviderMetricSample) {
			s.StandardMetric = thirdparty.StandardMetric_STANDARD_METRIC_UNSPECIFIED
		}},
		{"金额指标币种空", func(s *thirdparty.ProviderMetricSample) {
			s.StandardMetric = thirdparty.StandardMetric_STANDARD_METRIC_SPEND
		}},
		{"币种小写", func(s *thirdparty.ProviderMetricSample) {
			s.StandardMetric = thirdparty.StandardMetric_STANDARD_METRIC_SPEND
			s.Currency = "cny"
		}},
		{"币种非三字母", func(s *thirdparty.ProviderMetricSample) {
			s.StandardMetric = thirdparty.StandardMetric_STANDARD_METRIC_REVENUE
			s.Currency = "CN"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validReq()
			sample := validSample()
			tc.mutate(sample)
			if tc.name == "nil 样本" {
				r.Samples = []*thirdparty.ProviderMetricSample{nil}
			} else {
				r.Samples = []*thirdparty.ProviderMetricSample{sample}
			}
			if _, err := BuildBatch(r); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("期望 ErrInvalidRequest，得到 %v", err)
			}
		})
	}
	// 合法金额指标（币种 3 位大写）应通过
	r := validReq()
	r.Samples[0].StandardMetric = thirdparty.StandardMetric_STANDARD_METRIC_SPEND
	r.Samples[0].Currency = "CNY"
	if _, err := BuildBatch(r); err != nil {
		t.Fatalf("合法金额指标应通过: %v", err)
	}
}

func TestBuildBatchCountAndSlotValidation(t *testing.T) {
	// 样本数为 0 / 201 非法；200 合法
	if _, err := BuildBatch(&thirdparty.IngestProviderMetricsReq{}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("空请求应报 ErrInvalidRequest")
	}
	r := validReq()
	r.Samples = make([]*thirdparty.ProviderMetricSample, 201)
	for i := range r.Samples {
		s := validSample()
		s.ExternalObjectId = fmt.Sprintf("camp-%03d", i)
		r.Samples[i] = s
	}
	if _, err := BuildBatch(r); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("201 样本应报 ErrInvalidRequest")
	}
	// 200 个合法样本（槽位互异）应通过
	r200 := validReq()
	r200.Samples = make([]*thirdparty.ProviderMetricSample, 200)
	for i := range r200.Samples {
		s := validSample()
		s.ExternalObjectId = fmt.Sprintf("camp-%03d", i)
		s.BucketStart = timestamppb.New(time.Unix(int64(8*3600+3600*(i/10)), 0).UTC())
		r200.Samples[i] = s
	}
	if _, err := BuildBatch(r200); err != nil {
		t.Fatalf("200 样本应通过: %v", err)
	}
	// 槽位重复：同 object/metric/start/seconds/dimension 两样本 → 非法
	rDup := validReq()
	dup := validSample()
	rDup.Samples = []*thirdparty.ProviderMetricSample{validSample(), dup}
	if _, err := BuildBatch(rDup); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("槽位重复应报 ErrInvalidRequest")
	}
	// 维度不同 → 不同槽位，合法
	rDim := validReq()
	other := validSample()
	other.DimensionKey = "cn"
	rDim.Samples = []*thirdparty.ProviderMetricSample{validSample(), other}
	if _, err := BuildBatch(rDim); err != nil {
		t.Fatalf("不同维度槽位应通过: %v", err)
	}
	// day 桶（86400）合法
	rDay := validReq()
	rDay.Samples[0].BucketSeconds = 86400
	rDay.Samples[0].BucketStart = timestamppb.New(time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC))
	if _, err := BuildBatch(rDay); err != nil {
		t.Fatalf("day 桶应通过: %v", err)
	}
}
