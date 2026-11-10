package core

import (
	"encoding/json"
	"fmt"
	"regexp"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils"
	"google.golang.org/protobuf/encoding/protojson"
)

// M14d 控制台活动：server_v2 console 形状（marketing.CampaignConfig 消息，protojson）↔
// M11a 玩家形状（campaigns.config_json / campaign_redemption_codes.prize_config /
// campaign_wheel_slots.prize_config）的转换与校验。纯函数（无 DB/网络），单测覆盖。
// 玩家形状 JSON 字段与 logic/campaignOps.go 的 prizeCfg/signInConfig/wheelConfig 标签一一对应。

var campaignKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,63}$`)

// playerPrize 玩家形状奖品条目（与 logic.prizeCfg 同构，JSON 标签必须保持一致）
type playerPrize struct {
	Kind             int64  `json:"kind"`
	CashFen          int64  `json:"cashFen"`
	Points           int64  `json:"points"`
	CouponTemplateNo string `json:"couponTemplateNo,omitempty"`
	BoxId            int64  `json:"boxId"`
}

type playerSignInDay struct {
	Prizes []playerPrize `json:"prizes"`
}

type playerSignInConfig struct {
	Rewards []playerSignInDay `json:"rewards"`
}

type playerWheelConfig struct {
	DailyLimit int64 `json:"dailyLimit"`
}

// MaterializedRedeemCode 兑换码物化行（campaign_redemption_codes）
type MaterializedRedeemCode struct {
	Code           string
	MaxRedemptions int64
	PerUserLimit   int64
	PrizeConfig    string // 奖品数组 JSON（[]playerPrize）
}

// MaterializedWheelSlot 转盘槽物化行（campaign_wheel_slots）。
// 单奖品列填首个奖品（玩家展示/兜底，Kind/CashFen/PointsAmount/CouponTemplateNo）；
// PrizeConfig 存全量数组（wheelTx 多奖品发放依据）。
type MaterializedWheelSlot struct {
	SlotIndex        int64
	Label            string
	Weight           int64
	PrizeKind        int64  // 首奖品 kind（1 CASH 2 POINTS 3 COUPON 4 GACHA_BOX）
	CashFen          int64  // 首奖品现金（分）
	PointsAmount     int64  // 首奖品积分
	CouponTemplateNo string // 首奖品券模板单号
	PrizeConfig      string // 奖品数组 JSON（[]playerPrize，console 槽一律此形态）
	DailyStock       int64  // -1 不限；>0 按日库存（跨日重置）
	Remaining        int64  // 总库存（console 槽 -1 不限）
}

// MaterializedCampaign 发布物化结果
type MaterializedCampaign struct {
	ConfigJSON string // campaigns.config_json（玩家形状）
	Codes      []MaterializedRedeemCode
	Slots      []MaterializedWheelSlot
}

// ValidateCampaignDraft 校验新建/追加草稿请求（设计 D2，照抄 server_v2 NormalizeDraft + kind 校验）。
// 新建（campaignId==0）时 key 必填且匹配；追加时 key 允许为空（kind/key 固定由 repo 锁头校验）。
// 返回错误仅含校验语义，logic 层统一映射 5007。
func ValidateCampaignDraft(req *marketing.CreateCampaignDraftRequest) error {
	if req.TenantId == 0 {
		return fmt.Errorf("tenantId 必须大于 0")
	}
	if req.CampaignId == 0 && !campaignKeyRe.MatchString(req.CampaignKey) {
		return fmt.Errorf("campaignKey 非法（须匹配 ^[a-z][a-z0-9_.-]{1,63}$）")
	}
	if len(req.Name) == 0 || len(req.Name) > 128 {
		return fmt.Errorf("name 长度须在 1..128")
	}
	if req.StartAtUnixMillis <= 0 || req.EndAtUnixMillis <= 0 || req.EndAtUnixMillis <= req.StartAtUnixMillis {
		return fmt.Errorf("活动窗口非法（须 end>start>0）")
	}
	switch req.Kind {
	case marketing.CampaignKind_CAMPAIGN_KIND_SIGN_IN,
		marketing.CampaignKind_CAMPAIGN_KIND_REDEMPTION_CODE,
		marketing.CampaignKind_CAMPAIGN_KIND_PRIZE_WHEEL:
	default:
		return fmt.Errorf("不支持的 CampaignKind: %s", req.Kind)
	}
	_, err := MaterializeCampaignConfig(req.Kind, req.Config)
	return err
}

// MaterializeCampaignConfig console 配置 → 玩家形状物化（发布时落库用；含按 kind 的配置校验）。
func MaterializeCampaignConfig(kind marketing.CampaignKind, cfg *marketing.CampaignConfig) (*MaterializedCampaign, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config 不能为空")
	}
	switch kind {
	case marketing.CampaignKind_CAMPAIGN_KIND_SIGN_IN:
		s, err := materializeSignIn(cfg.SignIn)
		if err != nil {
			return nil, err
		}
		return &MaterializedCampaign{ConfigJSON: s}, nil
	case marketing.CampaignKind_CAMPAIGN_KIND_REDEMPTION_CODE:
		codes, err := materializeRedemption(cfg.Redemption)
		if err != nil {
			return nil, err
		}
		return &MaterializedCampaign{ConfigJSON: "{}", Codes: codes}, nil
	case marketing.CampaignKind_CAMPAIGN_KIND_PRIZE_WHEEL:
		s, slots, err := materializeWheel(cfg.PrizeWheel)
		if err != nil {
			return nil, err
		}
		return &MaterializedCampaign{ConfigJSON: s, Slots: slots}, nil
	default:
		return nil, fmt.Errorf("不支持的 CampaignKind: %s", kind)
	}
}

// materializeSignIn 签到：day 从 1 起连续；loyaltyPoints 物化为追加积分奖品
// （玩家引擎无每日积分字段，积分走同一发奖管线，处理方式优化）。
func materializeSignIn(cfg *marketing.SignInConfig) (string, error) {
	if cfg == nil || len(cfg.Rewards) == 0 {
		return "", fmt.Errorf("签到 rewards 不能为空")
	}
	rewards := make([]playerSignInDay, 0, len(cfg.Rewards))
	for i, r := range cfg.Rewards {
		if int64(r.Day) != int64(i)+1 {
			return "", fmt.Errorf("签到 day 须从 1 起连续（第 %d 项 day=%d）", i+1, r.Day)
		}
		if r.LoyaltyPoints < 0 {
			return "", fmt.Errorf("签到第 %d 天 loyaltyPoints 不能为负", r.Day)
		}
		var prizes []playerPrize
		for _, p := range r.Prizes {
			ps, err := convertPrize(p)
			if err != nil {
				return "", fmt.Errorf("签到第 %d 天: %w", r.Day, err)
			}
			prizes = append(prizes, ps...)
		}
		if r.LoyaltyPoints > 0 {
			prizes = append(prizes, playerPrize{Kind: 2, Points: r.LoyaltyPoints})
		}
		rewards = append(rewards, playerSignInDay{Prizes: prizes})
	}
	b, err := json.Marshal(playerSignInConfig{Rewards: rewards})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// materializeRedemption 兑换码：逐码校验 + 物化（loyaltyPoints 物化为追加积分奖品）
func materializeRedemption(cfg *marketing.RedemptionConfig) ([]MaterializedRedeemCode, error) {
	if cfg == nil || len(cfg.Codes) == 0 {
		return nil, fmt.Errorf("兑换码 codes 不能为空")
	}
	codes := make([]MaterializedRedeemCode, 0, len(cfg.Codes))
	for _, c := range cfg.Codes {
		if c.Code == "" || len(c.Code) > 64 {
			return nil, fmt.Errorf("兑换码 code 须非空且 ≤64 字符")
		}
		if c.MaxRedemptions <= 0 {
			return nil, fmt.Errorf("兑换码 %s maxRedemptions 必须大于 0", c.Code)
		}
		if c.PerUserLimit <= 0 {
			return nil, fmt.Errorf("兑换码 %s perUserLimit 必须大于 0", c.Code)
		}
		if c.PerUserLimit > c.MaxRedemptions {
			return nil, fmt.Errorf("兑换码 %s perUserLimit 不能大于 maxRedemptions", c.Code)
		}
		if c.LoyaltyPoints < 0 {
			return nil, fmt.Errorf("兑换码 %s loyaltyPoints 不能为负", c.Code)
		}
		var prizes []playerPrize
		for _, p := range c.Prizes {
			ps, err := convertPrize(p)
			if err != nil {
				return nil, fmt.Errorf("兑换码 %s: %w", c.Code, err)
			}
			prizes = append(prizes, ps...)
		}
		if c.LoyaltyPoints > 0 {
			prizes = append(prizes, playerPrize{Kind: 2, Points: c.LoyaltyPoints})
		}
		if len(prizes) == 0 {
			return nil, fmt.Errorf("兑换码 %s 无奖品", c.Code)
		}
		b, err := json.Marshal(prizes)
		if err != nil {
			return nil, err
		}
		codes = append(codes, MaterializedRedeemCode{
			Code:           c.Code,
			MaxRedemptions: int64(c.MaxRedemptions),
			PerUserLimit:   int64(c.PerUserLimit),
			PrizeConfig:    string(b),
		})
	}
	return codes, nil
}

// materializeWheel 转盘：槽全量奖品物化 prize_config；dailyStock 0→-1（不限）。
func materializeWheel(cfg *marketing.PrizeWheelConfig) (string, []MaterializedWheelSlot, error) {
	if cfg == nil || len(cfg.Slots) == 0 {
		return "", nil, fmt.Errorf("转盘 slots 不能为空")
	}
	if cfg.DailyLimit <= 0 {
		return "", nil, fmt.Errorf("转盘 dailyLimit 必须大于 0")
	}
	slots := make([]MaterializedWheelSlot, 0, len(cfg.Slots))
	for i, s := range cfg.Slots {
		if s.Weight <= 0 {
			return "", nil, fmt.Errorf("转盘槽 %d weight 必须大于 0", i)
		}
		if len(s.Prizes) == 0 {
			return "", nil, fmt.Errorf("转盘槽 %d prizes 不能为空", i)
		}
		var prizes []playerPrize
		for _, p := range s.Prizes {
			ps, err := convertPrize(p)
			if err != nil {
				return "", nil, fmt.Errorf("转盘槽 %d: %w", i, err)
			}
			prizes = append(prizes, ps...)
		}
		b, err := json.Marshal(prizes)
		if err != nil {
			return "", nil, err
		}
		dailyStock := int64(s.DailyStock)
		if dailyStock == 0 {
			dailyStock = -1
		}
		slots = append(slots, MaterializedWheelSlot{
			SlotIndex:        int64(i),
			Label:            wheelLabelOf(prizes[0]),
			Weight:           int64(s.Weight),
			PrizeKind:        prizes[0].Kind,
			CashFen:          prizes[0].CashFen,
			PointsAmount:     prizes[0].Points,
			CouponTemplateNo: prizes[0].CouponTemplateNo,
			PrizeConfig:      string(b),
			DailyStock:       dailyStock,
			Remaining:        -1, // console 槽总库存不限，按日库存由 daily_spent 管控
		})
	}
	b, err := json.Marshal(playerWheelConfig{DailyLimit: int64(cfg.DailyLimit)})
	if err != nil {
		return "", nil, err
	}
	return string(b), slots, nil
}

// convertPrize console Prize → 玩家形状奖品条目（count 重复展开，0 视为 1）。
// 仅支持 CASH/LOYALTY_POINT/COUPON/GACHA_BOX（其余 PrizeKind 本里程碑不支持）。
func convertPrize(p *marketing.Prize) ([]playerPrize, error) {
	count := int64(p.Count)
	if count <= 0 {
		count = 1
	}
	var out playerPrize
	switch p.Kind {
	case marketing.PrizeKind_PRIZE_KIND_CASH:
		if p.Amount <= 0 {
			return nil, fmt.Errorf("现金奖品 amount 必须大于 0")
		}
		out = playerPrize{Kind: 1, CashFen: p.Amount}
	case marketing.PrizeKind_PRIZE_KIND_LOYALTY_POINT:
		if p.Amount <= 0 {
			return nil, fmt.Errorf("积分奖品 amount 必须大于 0")
		}
		out = playerPrize{Kind: 2, Points: p.Amount}
	case marketing.PrizeKind_PRIZE_KIND_COUPON:
		if p.RefId == 0 {
			return nil, fmt.Errorf("券奖品 refId（券模板ID）必须大于 0")
		}
		out = playerPrize{Kind: 3, CouponTemplateNo: ConsoleTemplateNo(p.RefId)}
	case marketing.PrizeKind_PRIZE_KIND_GACHA_BOX:
		if p.RefId == 0 {
			return nil, fmt.Errorf("宝箱奖品 refId（boxId）必须大于 0")
		}
		out = playerPrize{Kind: 4, BoxId: int64(p.RefId)}
	default:
		return nil, fmt.Errorf("不支持的 PrizeKind: %s", p.Kind)
	}
	prizes := make([]playerPrize, 0, count)
	for i := int64(0); i < count; i++ {
		prizes = append(prizes, out)
	}
	return prizes, nil
}

// wheelLabelOf 玩家展示用槽名（console WeightedSlot 无 label 字段，由首奖品推导）
func wheelLabelOf(p playerPrize) string {
	switch p.Kind {
	case 1:
		return fmt.Sprintf("现金 %s 元", utils.FenToYuan(p.CashFen))
	case 2:
		return fmt.Sprintf("积分 %d", p.Points)
	case 3:
		return "优惠券"
	case 4:
		return "宝箱资格"
	}
	return "奖品"
}

// MarshalCampaignConfig console 形状 protojson（camelCase，campaign_versions.config_json 存储用）。
func MarshalCampaignConfig(cfg *marketing.CampaignConfig) (string, error) {
	b, err := protojson.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UnmarshalCampaignConfig 解析存储的 console 形状 protojson。
func UnmarshalCampaignConfig(s string) (*marketing.CampaignConfig, error) {
	var cfg marketing.CampaignConfig
	if err := protojson.Unmarshal([]byte(s), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
