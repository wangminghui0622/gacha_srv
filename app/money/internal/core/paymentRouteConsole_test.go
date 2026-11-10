package core

import (
	"testing"

	"gacha_srv/app/money/money"
)

// goodAuto 新建 AUTO 合法请求
func goodAuto() *money.PutPaymentRouteRequest {
	return &money.PutPaymentRouteRequest{
		OperatorUserId: 7,
		Policy: &money.PaymentRoutePolicy{
			TenantId: 1,
			Channel:  money.PaymentChannel_PAYMENT_CHANNEL_WECHAT,
			Scene:    money.PaymentScene_PAYMENT_SCENE_APP,
			Mode:     money.PaymentRouteMode_PAYMENT_ROUTE_MODE_AUTO,
		},
		Candidates: []*money.PaymentRouteCandidate{
			{Provider: money.PaymentProvider_PAYMENT_PROVIDER_NATIVE_WECHAT, MerchantAccountId: "mch-wx-001", Weight: 100, Enabled: true},
			{Provider: money.PaymentProvider_PAYMENT_PROVIDER_NATIVE_ALIPAY, MerchantAccountId: "mch-al-001", Weight: 50, Enabled: false},
		},
	}
}

func TestValidatePutPaymentRoute_Good(t *testing.T) {
	// 新建 AUTO
	if err := ValidatePutPaymentRoute(goodAuto()); err != nil {
		t.Fatalf("新建 AUTO 应合法: %v", err)
	}
	// 新建 PINNED（pinned 在 enabled 中）
	req := goodAuto()
	req.Policy.Mode = money.PaymentRouteMode_PAYMENT_ROUTE_MODE_PINNED
	req.Policy.PinnedProvider = money.PaymentProvider_PAYMENT_PROVIDER_NATIVE_WECHAT
	req.Policy.PinnedMerchantAccountId = "mch-wx-001"
	if err := ValidatePutPaymentRoute(req); err != nil {
		t.Fatalf("新建 PINNED 应合法: %v", err)
	}
	// 更新（policyId 非 0，expectedVersion 一致）
	upd := goodAuto()
	upd.Policy.PolicyId = 5
	upd.Policy.Version = 3
	upd.ExpectedVersion = 3
	if err := ValidatePutPaymentRoute(upd); err != nil {
		t.Fatalf("更新应合法: %v", err)
	}
}

func TestValidatePutPaymentRoute_Bad(t *testing.T) {
	if err := ValidatePutPaymentRoute(nil); err == nil {
		t.Fatal("nil 请求应报错")
	}
	cases := []struct {
		name string
		mut  func(*money.PutPaymentRouteRequest)
	}{
		{"operator 为 0", func(r *money.PutPaymentRouteRequest) { r.OperatorUserId = 0 }},
		{"tenant 非 1", func(r *money.PutPaymentRouteRequest) { r.Policy.TenantId = 2 }},
		{"channel 非法", func(r *money.PutPaymentRouteRequest) { r.Policy.Channel = money.PaymentChannel_PAYMENT_CHANNEL_UNSPECIFIED }},
		{"scene 非法", func(r *money.PutPaymentRouteRequest) { r.Policy.Scene = money.PaymentScene_PAYMENT_SCENE_UNSPECIFIED }},
		{"mode 非法", func(r *money.PutPaymentRouteRequest) { r.Policy.Mode = money.PaymentRouteMode_PAYMENT_ROUTE_MODE_UNSPECIFIED }},
		{"candidates 为空", func(r *money.PutPaymentRouteRequest) { r.Candidates = nil }},
		{"candidates 超 16", func(r *money.PutPaymentRouteRequest) {
			cs := make([]*money.PaymentRouteCandidate, 17)
			for i := range cs {
				cs[i] = &money.PaymentRouteCandidate{Provider: money.PaymentProvider_PAYMENT_PROVIDER_NATIVE_WECHAT, MerchantAccountId: "m", Weight: 1, Enabled: true}
			}
			r.Candidates = cs
		}},
		{"新建 expectedVersion 非 0", func(r *money.PutPaymentRouteRequest) { r.ExpectedVersion = 1 }},
		{"新建 policy.version 非 0", func(r *money.PutPaymentRouteRequest) { r.Policy.Version = 1 }},
		{"更新 expectedVersion 为 0", func(r *money.PutPaymentRouteRequest) {
			r.Policy.PolicyId = 5
			r.ExpectedVersion = 0
		}},
		{"更新 version 与 expectedVersion 不符", func(r *money.PutPaymentRouteRequest) {
			r.Policy.PolicyId = 5
			r.Policy.Version = 2
			r.ExpectedVersion = 3
		}},
		{"AUTO 带 pinned", func(r *money.PutPaymentRouteRequest) {
			r.Policy.PinnedProvider = money.PaymentProvider_PAYMENT_PROVIDER_NATIVE_WECHAT
			r.Policy.PinnedMerchantAccountId = "mch-wx-001"
		}},
		{"PINNED 缺 pinned", func(r *money.PutPaymentRouteRequest) {
			r.Policy.Mode = money.PaymentRouteMode_PAYMENT_ROUTE_MODE_PINNED
		}},
		{"candidate 带 candidateId", func(r *money.PutPaymentRouteRequest) { r.Candidates[0].CandidateId = 9 }},
		{"candidate 带 policyId", func(r *money.PutPaymentRouteRequest) { r.Candidates[0].PolicyId = 9 }},
		{"candidate provider 非法", func(r *money.PutPaymentRouteRequest) {
			r.Candidates[0].Provider = money.PaymentProvider_PAYMENT_PROVIDER_HUIFU
		}},
		{"merchant 为空", func(r *money.PutPaymentRouteRequest) { r.Candidates[0].MerchantAccountId = "" }},
		{"merchant 超 64", func(r *money.PutPaymentRouteRequest) {
			r.Candidates[0].MerchantAccountId = "mch-00000000000000000000000000000000000000000000000000000000000000001"
		}},
		{"merchant 带首尾空白", func(r *money.PutPaymentRouteRequest) {
			r.Candidates[0].MerchantAccountId = " mch-wx-001"
		}},
		{"weight 为 0", func(r *money.PutPaymentRouteRequest) { r.Candidates[0].Weight = 0 }},
		{"weight 超 10000", func(r *money.PutPaymentRouteRequest) { r.Candidates[0].Weight = 10001 }},
		{"候选重复", func(r *money.PutPaymentRouteRequest) {
			r.Candidates = append(r.Candidates, &money.PaymentRouteCandidate{Provider: money.PaymentProvider_PAYMENT_PROVIDER_NATIVE_WECHAT, MerchantAccountId: "mch-wx-001", Weight: 1, Enabled: true})
		}},
		{"无 enabled", func(r *money.PutPaymentRouteRequest) { r.Candidates[0].Enabled = false }},
		{"PINNED 的 pinned 不在 enabled", func(r *money.PutPaymentRouteRequest) {
			r.Policy.Mode = money.PaymentRouteMode_PAYMENT_ROUTE_MODE_PINNED
			r.Policy.PinnedProvider = money.PaymentProvider_PAYMENT_PROVIDER_NATIVE_WECHAT
			r.Policy.PinnedMerchantAccountId = "mch-al-001" // disabled 候选
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := goodAuto()
			tc.mut(req)
			if err := ValidatePutPaymentRoute(req); err == nil {
				t.Fatalf("应报错")
			}
		})
	}
}

func TestValidateGetPaymentRoute(t *testing.T) {
	if err := ValidateGetPaymentRoute(&money.GetPaymentRouteRequest{
		Channel: money.PaymentChannel_PAYMENT_CHANNEL_WECHAT,
		Scene:   money.PaymentScene_PAYMENT_SCENE_MINI_PROGRAM,
	}); err != nil {
		t.Fatalf("合法 GET 应通过: %v", err)
	}
	if err := ValidateGetPaymentRoute(nil); err == nil {
		t.Fatal("nil 请求应报错")
	}
	if err := ValidateGetPaymentRoute(&money.GetPaymentRouteRequest{
		Channel: money.PaymentChannel_PAYMENT_CHANNEL_UNSPECIFIED,
		Scene:   money.PaymentScene_PAYMENT_SCENE_APP,
	}); err == nil {
		t.Fatal("channel 非法应报错")
	}
	if err := ValidateGetPaymentRoute(&money.GetPaymentRouteRequest{
		Channel: money.PaymentChannel_PAYMENT_CHANNEL_WECHAT,
		Scene:   money.PaymentScene_PAYMENT_SCENE_UNSPECIFIED,
	}); err == nil {
		t.Fatal("scene 非法应报错")
	}
}
