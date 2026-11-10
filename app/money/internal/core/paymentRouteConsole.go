// Package core M14d 控制台纯函数（payment 域）：支付路由存管校验（D6，照抄 server_v2
// ValidateRoutePolicy，registry.GetConfigured 检查按设计偏离省略——本仓适配器固定，
// provider 白名单即校验）。
package core

import (
	"fmt"
	"strings"

	"gacha_srv/app/money/money"
)

// ValidatePutPaymentRoute 校验 PUT 请求（2009 CodePaymentRouteInvalid）。
// 新建：policyId==0 且 expectedVersion==0；更新：policyId==行 id 且 expectedVersion==行 version。
func ValidatePutPaymentRoute(req *money.PutPaymentRouteRequest) error {
	if req == nil || req.Policy == nil || req.OperatorUserId == 0 {
		return fmt.Errorf("policy/operatorUserId 不能为空")
	}
	p := req.Policy
	if p.TenantId != 1 {
		return fmt.Errorf("tenantId 须为 1")
	}
	if p.Channel != money.PaymentChannel_PAYMENT_CHANNEL_WECHAT &&
		p.Channel != money.PaymentChannel_PAYMENT_CHANNEL_ALIPAY {
		return fmt.Errorf("channel 须为 WECHAT/ALIPAY")
	}
	if p.Scene != money.PaymentScene_PAYMENT_SCENE_APP &&
		p.Scene != money.PaymentScene_PAYMENT_SCENE_MINI_PROGRAM &&
		p.Scene != money.PaymentScene_PAYMENT_SCENE_H5 {
		return fmt.Errorf("scene 须为 APP/MINI_PROGRAM/H5")
	}
	if p.Mode != money.PaymentRouteMode_PAYMENT_ROUTE_MODE_AUTO &&
		p.Mode != money.PaymentRouteMode_PAYMENT_ROUTE_MODE_PINNED {
		return fmt.Errorf("mode 须为 AUTO/PINNED")
	}
	if len(req.Candidates) == 0 || len(req.Candidates) > 16 {
		return fmt.Errorf("candidates 须 1..16 条")
	}
	// 新建/更新分叉（server_v2 同款）
	if p.PolicyId == 0 && (req.ExpectedVersion != 0 || p.Version != 0) {
		return fmt.Errorf("新建路由 expectedVersion/policy.version 须为 0")
	}
	if p.PolicyId != 0 && (req.ExpectedVersion == 0 || p.Version != req.ExpectedVersion) {
		return fmt.Errorf("更新路由 policy.version 须等于 expectedVersion")
	}
	if p.Mode == money.PaymentRouteMode_PAYMENT_ROUTE_MODE_AUTO &&
		(p.PinnedProvider != 0 || p.PinnedMerchantAccountId != "") {
		return fmt.Errorf("AUTO 模式不得携带 pinned")
	}
	if p.Mode == money.PaymentRouteMode_PAYMENT_ROUTE_MODE_PINNED &&
		(p.PinnedProvider == 0 || p.PinnedMerchantAccountId == "") {
		return fmt.Errorf("PINNED 模式须指定 pinnedProvider 与 pinnedMerchantAccountId")
	}

	seen := make(map[string]bool, len(req.Candidates))
	enabled := 0
	pinnedHit := false
	for _, c := range req.Candidates {
		if c == nil || c.CandidateId != 0 || c.PolicyId != 0 {
			return fmt.Errorf("candidate 的 candidateId/policyId 须为 0")
		}
		// 本仓适配器固定：provider 白名单即校验（偏离记录见 m14d-console.md D6）
		if c.Provider != money.PaymentProvider_PAYMENT_PROVIDER_NATIVE_WECHAT &&
			c.Provider != money.PaymentProvider_PAYMENT_PROVIDER_NATIVE_ALIPAY {
			return fmt.Errorf("candidate provider 须为 NATIVE_WECHAT/NATIVE_ALIPAY")
		}
		m := c.MerchantAccountId
		if m == "" || len(m) > 64 || strings.TrimSpace(m) != m {
			return fmt.Errorf("candidate merchantAccountId 须 1..64 字符且无首尾空白")
		}
		if c.Weight == 0 || c.Weight > 10000 {
			return fmt.Errorf("candidate weight 须 1..10000")
		}
		key := fmt.Sprintf("%d:%s", c.Provider, m)
		if seen[key] {
			return fmt.Errorf("candidate (provider,merchant) 重复: %s", key)
		}
		seen[key] = true
		if c.Enabled {
			enabled++
			if p.Mode == money.PaymentRouteMode_PAYMENT_ROUTE_MODE_PINNED &&
				c.Provider == p.PinnedProvider && c.MerchantAccountId == p.PinnedMerchantAccountId {
				pinnedHit = true
			}
		}
	}
	if enabled == 0 {
		return fmt.Errorf("candidates 至少一条 enabled")
	}
	if p.Mode == money.PaymentRouteMode_PAYMENT_ROUTE_MODE_PINNED && !pinnedHit {
		return fmt.Errorf("PINNED 的 pinned 候选必须在 enabled 中")
	}
	return nil
}

// ValidateGetPaymentRoute 校验 GET 请求（channel/scene 枚举）
func ValidateGetPaymentRoute(req *money.GetPaymentRouteRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Channel != money.PaymentChannel_PAYMENT_CHANNEL_WECHAT &&
		req.Channel != money.PaymentChannel_PAYMENT_CHANNEL_ALIPAY {
		return fmt.Errorf("channel 须为 WECHAT/ALIPAY")
	}
	if req.Scene != money.PaymentScene_PAYMENT_SCENE_APP &&
		req.Scene != money.PaymentScene_PAYMENT_SCENE_MINI_PROGRAM &&
		req.Scene != money.PaymentScene_PAYMENT_SCENE_H5 {
		return fmt.Errorf("scene 须为 APP/MINI_PROGRAM/H5")
	}
	return nil
}
