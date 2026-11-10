package core

import (
	"strings"
	"testing"

	"gacha_srv/app/marketing/marketing"
	"google.golang.org/protobuf/proto"
)

func TestValidateCampaignDraft(t *testing.T) {
	base := &marketing.CreateCampaignDraftRequest{
		TenantId:          1,
		CampaignKey:       "test.key_1",
		Kind:              marketing.CampaignKind_CAMPAIGN_KIND_SIGN_IN,
		Name:              "测试活动",
		StartAtUnixMillis: 1000,
		EndAtUnixMillis:   2000,
		Config: &marketing.CampaignConfig{
			SignIn: &marketing.SignInConfig{Rewards: []*marketing.SignInReward{{Day: 1, LoyaltyPoints: 10}}},
		},
	}
	if err := ValidateCampaignDraft(base); err != nil {
		t.Fatalf("合法草稿被拒: %v", err)
	}
	// 追加模式（campaignId>0）key 可空
	app := proto.Clone(base).(*marketing.CreateCampaignDraftRequest)
	app.CampaignId = 5
	app.CampaignKey = ""
	if err := ValidateCampaignDraft(app); err != nil {
		t.Fatalf("追加模式合法草稿被拒: %v", err)
	}
	cases := []struct {
		name string
		mut  func(*marketing.CreateCampaignDraftRequest)
	}{
		{"key 非法", func(r *marketing.CreateCampaignDraftRequest) { r.CampaignKey = "Bad Key" }},
		{"key 过短", func(r *marketing.CreateCampaignDraftRequest) { r.CampaignKey = "a" }},
		{"name 空", func(r *marketing.CreateCampaignDraftRequest) { r.Name = "" }},
		{"窗口倒挂", func(r *marketing.CreateCampaignDraftRequest) { r.EndAtUnixMillis = 1000 }},
		{"kind 不支持", func(r *marketing.CreateCampaignDraftRequest) {
			r.Kind = marketing.CampaignKind_CAMPAIGN_KIND_UNSPECIFIED
		}},
		{"config 空", func(r *marketing.CreateCampaignDraftRequest) { r.Config = nil }},
	}
	for _, c := range cases {
		r := proto.Clone(base).(*marketing.CreateCampaignDraftRequest)
		c.mut(r)
		if err := ValidateCampaignDraft(r); err == nil {
			t.Errorf("%s: 应报错", c.name)
		}
	}
}

func TestMaterializeSignIn(t *testing.T) {
	cfg := &marketing.CampaignConfig{SignIn: &marketing.SignInConfig{Rewards: []*marketing.SignInReward{
		{Day: 1, LoyaltyPoints: 10, Prizes: []*marketing.Prize{
			{Kind: marketing.PrizeKind_PRIZE_KIND_CASH, Amount: 100},
			{Kind: marketing.PrizeKind_PRIZE_KIND_COUPON, RefId: 7, Count: 2},
		}},
		{Day: 2, LoyaltyPoints: 20},
	}}}
	m, err := MaterializeCampaignConfig(marketing.CampaignKind_CAMPAIGN_KIND_SIGN_IN, cfg)
	if err != nil {
		t.Fatalf("物化失败: %v", err)
	}
	if !strings.Contains(m.ConfigJSON, `"rewards"`) {
		t.Fatalf("缺少 rewards 字段: %s", m.ConfigJSON)
	}
	if !strings.Contains(m.ConfigJSON, `"kind":2,"cashFen":0,"points":10,"boxId":0`) {
		t.Errorf("loyaltyPoints 未物化为积分奖品: %s", m.ConfigJSON)
	}
	if !strings.Contains(m.ConfigJSON, `"kind":3,"cashFen":0,"points":0,"couponTemplateNo":"CONSOLE-7"`) {
		t.Errorf("券奖品 refId 未转模板号: %s", m.ConfigJSON)
	}
	if !strings.Contains(m.ConfigJSON, `"kind":2,"cashFen":0,"points":20,"boxId":0`) {
		t.Errorf("第 2 天积分未物化: %s", m.ConfigJSON)
	}
	// day 不连续
	bad := &marketing.CampaignConfig{SignIn: &marketing.SignInConfig{Rewards: []*marketing.SignInReward{{Day: 2}}}}
	if _, err := MaterializeCampaignConfig(marketing.CampaignKind_CAMPAIGN_KIND_SIGN_IN, bad); err == nil {
		t.Error("day 不连续应报错")
	}
}

func TestMaterializeRedemption(t *testing.T) {
	cfg := &marketing.CampaignConfig{Redemption: &marketing.RedemptionConfig{Codes: []*marketing.RedemptionCodeSpec{
		{Code: "GOOD", MaxRedemptions: 10, PerUserLimit: 1, LoyaltyPoints: 5,
			Prizes: []*marketing.Prize{{Kind: marketing.PrizeKind_PRIZE_KIND_LOYALTY_POINT, Amount: 100}}},
	}}}
	m, err := MaterializeCampaignConfig(marketing.CampaignKind_CAMPAIGN_KIND_REDEMPTION_CODE, cfg)
	if err != nil {
		t.Fatalf("物化失败: %v", err)
	}
	if len(m.Codes) != 1 || m.Codes[0].MaxRedemptions != 10 || m.Codes[0].PerUserLimit != 1 {
		t.Fatalf("兑换码行错: %+v", m.Codes)
	}
	if !strings.Contains(m.Codes[0].PrizeConfig, `"kind":2,"cashFen":0,"points":5,"boxId":0`) {
		t.Errorf("loyaltyPoints 未物化: %s", m.Codes[0].PrizeConfig)
	}
	if m.ConfigJSON != "{}" {
		t.Errorf("兑换码 config_json 应为 {}: %s", m.ConfigJSON)
	}
	bad := &marketing.CampaignConfig{Redemption: &marketing.RedemptionConfig{Codes: []*marketing.RedemptionCodeSpec{
		{Code: "X", MaxRedemptions: 1, PerUserLimit: 2, Prizes: []*marketing.Prize{{Kind: marketing.PrizeKind_PRIZE_KIND_CASH, Amount: 1}}},
	}}}
	if _, err := MaterializeCampaignConfig(marketing.CampaignKind_CAMPAIGN_KIND_REDEMPTION_CODE, bad); err == nil {
		t.Error("perUserLimit>maxRedemptions 应报错")
	}
}

func TestMaterializeWheel(t *testing.T) {
	cfg := &marketing.CampaignConfig{PrizeWheel: &marketing.PrizeWheelConfig{
		DailyLimit: 3,
		Slots: []*marketing.WeightedSlot{
			{Weight: 60, DailyStock: 0, Prizes: []*marketing.Prize{
				{Kind: marketing.PrizeKind_PRIZE_KIND_CASH, Amount: 500},
				{Kind: marketing.PrizeKind_PRIZE_KIND_LOYALTY_POINT, Amount: 20},
			}},
			{Weight: 40, DailyStock: 10, Prizes: []*marketing.Prize{
				{Kind: marketing.PrizeKind_PRIZE_KIND_GACHA_BOX, RefId: 9},
			}},
		},
	}}
	m, err := MaterializeCampaignConfig(marketing.CampaignKind_CAMPAIGN_KIND_PRIZE_WHEEL, cfg)
	if err != nil {
		t.Fatalf("物化失败: %v", err)
	}
	if !strings.Contains(m.ConfigJSON, `"dailyLimit":3`) {
		t.Errorf("dailyLimit 错: %s", m.ConfigJSON)
	}
	if len(m.Slots) != 2 {
		t.Fatalf("槽数错: %d", len(m.Slots))
	}
	s0 := m.Slots[0]
	if s0.Weight != 60 || s0.DailyStock != -1 {
		t.Errorf("槽 0 权重/库存错（0 应转 -1）: %+v", s0)
	}
	if !strings.Contains(s0.PrizeConfig, `"kind":1,"cashFen":500,"points":0,"boxId":0`) || !strings.Contains(s0.PrizeConfig, `"kind":2,"cashFen":0,"points":20,"boxId":0`) {
		t.Errorf("槽 0 多奖品未物化: %s", s0.PrizeConfig)
	}
	if s0.Label != "现金 5.00 元" {
		t.Errorf("槽 0 label 错: %s", s0.Label)
	}
	if m.Slots[1].DailyStock != 10 || !strings.Contains(m.Slots[1].PrizeConfig, `"kind":4,"cashFen":0,"points":0,"boxId":9`) {
		t.Errorf("槽 1 错: %+v", m.Slots[1])
	}
	bad := &marketing.CampaignConfig{PrizeWheel: &marketing.PrizeWheelConfig{DailyLimit: 1, Slots: []*marketing.WeightedSlot{{Weight: 0}}}}
	if _, err := MaterializeCampaignConfig(marketing.CampaignKind_CAMPAIGN_KIND_PRIZE_WHEEL, bad); err == nil {
		t.Error("weight=0 应报错")
	}
}

func TestConvertPrizeCount(t *testing.T) {
	ps, err := convertPrize(&marketing.Prize{Kind: marketing.PrizeKind_PRIZE_KIND_CASH, Amount: 100, Count: 3})
	if err != nil || len(ps) != 3 {
		t.Fatalf("count=3 应展开 3 条: %v %d", err, len(ps))
	}
	ps, err = convertPrize(&marketing.Prize{Kind: marketing.PrizeKind_PRIZE_KIND_CASH, Amount: 100})
	if err != nil || len(ps) != 1 {
		t.Fatalf("count=0 应视为 1: %v %d", err, len(ps))
	}
	if _, err := convertPrize(&marketing.Prize{Kind: marketing.PrizeKind_PRIZE_KIND_CASH, Amount: 0}); err == nil {
		t.Error("amount=0 应报错")
	}
	if _, err := convertPrize(&marketing.Prize{Kind: marketing.PrizeKind_PRIZE_KIND_COUPON}); err == nil {
		t.Error("券 refId=0 应报错")
	}
}
