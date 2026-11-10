package core

import (
	"testing"

	"gacha_srv/app/marketing/marketing"
	"google.golang.org/protobuf/proto"
)

func validTemplate() *marketing.CouponTemplate {
	return &marketing.CouponTemplate{
		TenantId:    1,
		Kind:        marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_FIXED_MINOR,
		AmountMinor: 500,
		Currency:    "CNY",
		Funder:      marketing.DiscountFunder_DISCOUNT_FUNDER_PLATFORM,
	}
}

func TestValidateCouponTemplate(t *testing.T) {
	if err := ValidateCouponTemplate(validTemplate()); err != nil {
		t.Fatalf("合法模板被拒: %v", err)
	}
	// PERCENT_BPS 边界
	pct := validTemplate()
	pct.Kind = marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_PERCENT_BPS
	pct.AmountMinor = 0
	pct.PercentBps = 10000
	if err := ValidateCouponTemplate(pct); err != nil {
		t.Fatalf("percentBps=10000 应合法: %v", err)
	}
	pct.PercentBps = 10001
	if err := ValidateCouponTemplate(pct); err == nil {
		t.Error("percentBps>10000 应报错")
	}
	// CUSTOM_MINOR：amountMinor>0（面额上限）+ bps=0
	custom := validTemplate()
	custom.Kind = marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_CUSTOM_MINOR
	custom.AmountMinor = 1000
	if err := ValidateCouponTemplate(custom); err != nil {
		t.Fatalf("CUSTOM_MINOR 合法模板应通过: %v", err)
	}
	custom.AmountMinor = 0
	if err := ValidateCouponTemplate(custom); err == nil {
		t.Error("CUSTOM_MINOR amountMinor=0 应报错")
	}
	// FIXED 券带 bps 应报错
	fixedBps := validTemplate()
	fixedBps.PercentBps = 500
	if err := ValidateCouponTemplate(fixedBps); err == nil {
		t.Error("FIXED_MINOR 券 percentBps≠0 应报错")
	}
	// DRAW_REWARD 缺引用
	dr := validTemplate()
	dr.Kind = marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_DRAW_REWARD
	if err := ValidateCouponTemplate(dr); err == nil {
		t.Error("DRAW_REWARD 缺 rewardTemplateId 应报错")
	}
	dr.RewardTemplateId = 3
	dr.RewardCount = 1
	if err := ValidateCouponTemplate(dr); err != nil {
		t.Fatalf("DRAW_REWARD 完整模板应合法: %v", err)
	}
	// 通用校验
	bad := validTemplate()
	bad.Currency = "cny"
	if err := ValidateCouponTemplate(bad); err == nil {
		t.Error("小写 currency 应报错")
	}
	bad = validTemplate()
	bad.MinUnitPriceMinor = 200
	bad.MaxUnitPriceMinor = 100
	if err := ValidateCouponTemplate(bad); err == nil {
		t.Error("单价下限>上限应报错")
	}
}

func TestValidateGrantCoupons(t *testing.T) {
	req := &marketing.GrantCouponsRequest{
		RequestId:  "req-1",
		TenantId:   1,
		UserId:     11,
		TemplateId: 7,
		Count:      3,
		Source:     marketing.CouponGrantSource_COUPON_GRANT_SOURCE_CONSOLE,
	}
	if err := ValidateGrantCoupons(req); err != nil {
		t.Fatalf("合法请求被拒: %v", err)
	}
	cases := []struct {
		name string
		mut  func(*marketing.GrantCouponsRequest)
	}{
		{"requestId 空", func(r *marketing.GrantCouponsRequest) { r.RequestId = "" }},
		{"count 超上限", func(r *marketing.GrantCouponsRequest) { r.Count = 101 }},
		{"customAmount 负", func(r *marketing.GrantCouponsRequest) { r.CustomAmountMinor = -1 }},
		{"CAMPAIGN 缺 campaignId", func(r *marketing.GrantCouponsRequest) {
			r.Source = marketing.CouponGrantSource_COUPON_GRANT_SOURCE_CAMPAIGN
		}},
		{"source 非法", func(r *marketing.GrantCouponsRequest) {
			r.Source = marketing.CouponGrantSource_COUPON_GRANT_SOURCE_UNSPECIFIED
		}},
	}
	for _, c := range cases {
		r := proto.Clone(req).(*marketing.GrantCouponsRequest)
		c.mut(r)
		if err := ValidateGrantCoupons(r); err == nil {
			t.Errorf("%s: 应报错", c.name)
		}
	}
}

func TestCheckCustomAmount(t *testing.T) {
	tpl := validTemplate() // FIXED_MINOR
	if err := CheckCustomAmount(tpl, 0); err != nil {
		t.Errorf("非 CUSTOM_MINOR 券 custom=0 应合法: %v", err)
	}
	if err := CheckCustomAmount(tpl, 99); err == nil {
		t.Error("非 CUSTOM_MINOR 券 custom≠0 应报错")
	}
	custom := validTemplate()
	custom.Kind = marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_CUSTOM_MINOR
	custom.AmountMinor = 1000
	if err := CheckCustomAmount(custom, 500); err != nil {
		t.Fatalf("面额 500 ≤ 上限 1000 应合法: %v", err)
	}
	if err := CheckCustomAmount(custom, 1001); err == nil {
		t.Error("超上限应报错")
	}
	if err := CheckCustomAmount(custom, 0); err == nil {
		t.Error("CUSTOM_MINOR 券须指定面额，0 应报错")
	}
}

func TestCouponNos(t *testing.T) {
	if ConsoleTemplateNo(7) != "CONSOLE-7" {
		t.Errorf("ConsoleTemplateNo 错: %s", ConsoleTemplateNo(7))
	}
	if GrantCouponNo("r1", 0) != "gc_r1#0" {
		t.Errorf("GrantCouponNo 错: %s", GrantCouponNo("r1", 0))
	}
	if ExpiresAtUnixMillis(1000, 0) != 0 {
		t.Error("validityDays=0 应无期限")
	}
	if ExpiresAtUnixMillis(1000, 2) != 1000+2*86400000 {
		t.Error("到期时间错")
	}
}
