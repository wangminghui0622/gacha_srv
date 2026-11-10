package core

import (
	"fmt"
	"regexp"

	"gacha_srv/app/marketing/marketing"
)

// M14d 控制台券模板：发布校验（照抄 server_v2 NormalizeTemplate 语义）、
// 编号派生（console 模板号 / 发券幂等键）、发券请求校验。纯函数，单测覆盖。

var (
	currencyRe      = regexp.MustCompile(`^[A-Z]{3}$`)
	businessKeyRe   = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,64}$`)
	maxGrantCoupons = uint32(100)
)

// ConsoleTemplateNo console 发券引用的券模板号（"CONSOLE-<templateId>"，与 server_v2 约定的
// 编号空间不冲突：玩家侧模板为 "T-*" 前缀）
func ConsoleTemplateNo(templateId uint64) string {
	return fmt.Sprintf("CONSOLE-%d", templateId)
}

// GrantCouponNo 发券幂等来源键（sourceId = "gc_<requestId>#<序号>"，逐张唯一，
// user_coupons.uk_source_id 防重复发券）
func GrantCouponNo(requestId string, n int) string {
	return fmt.Sprintf("gc_%s#%d", requestId, n)
}

// ValidateCouponTemplate 发布券模板校验（server_v2 NormalizeTemplate：按 kind 的字段组合
// 校验 + 通用范围校验）。返回错误仅含校验语义，logic 层统一映射 5015/5007。
func ValidateCouponTemplate(t *marketing.CouponTemplate) error {
	if t == nil {
		return fmt.Errorf("template 不能为空")
	}
	if t.TenantId == 0 {
		return fmt.Errorf("tenantId 必须大于 0")
	}
	if len(t.Currency) == 0 || !currencyRe.MatchString(t.Currency) {
		return fmt.Errorf("currency 须为 3 位大写字母")
	}
	switch t.Funder {
	case marketing.DiscountFunder_DISCOUNT_FUNDER_PLATFORM, marketing.DiscountFunder_DISCOUNT_FUNDER_MERCHANT:
	default:
		return fmt.Errorf("不支持的 funder: %s", t.Funder)
	}
	if t.MinGrossMinor < 0 {
		return fmt.Errorf("minGrossMinor 不能为负")
	}
	if t.MinUnitPriceMinor < 0 || t.MaxUnitPriceMinor < 0 {
		return fmt.Errorf("单价上下限不能为负")
	}
	if t.MaxUnitPriceMinor > 0 && t.MinUnitPriceMinor > t.MaxUnitPriceMinor {
		return fmt.Errorf("minUnitPriceMinor 不能大于 maxUnitPriceMinor")
	}
	if t.MaxItemCount > 0 && t.MinItemCount > t.MaxItemCount {
		return fmt.Errorf("minItemCount 不能大于 maxItemCount")
	}
	if t.TargetBusinessKey != "" && !businessKeyRe.MatchString(t.TargetBusinessKey) {
		return fmt.Errorf("targetBusinessKey 非法（须匹配 ^[A-Za-z0-9_.:-]{1,64}$）")
	}
	switch t.Kind {
	case marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_FIXED_MINOR:
		if t.AmountMinor <= 0 {
			return fmt.Errorf("FIXED_MINOR 券 amountMinor 必须大于 0")
		}
		if t.PercentBps != 0 {
			return fmt.Errorf("FIXED_MINOR 券 percentBps 必须为 0")
		}
	case marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_PERCENT_BPS:
		if t.PercentBps <= 0 || t.PercentBps > 10000 {
			return fmt.Errorf("PERCENT_BPS 券 percentBps 须在 (0, 10000]")
		}
		if t.AmountMinor != 0 {
			return fmt.Errorf("PERCENT_BPS 券 amountMinor 必须为 0")
		}
	case marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_FREE:
		if t.AmountMinor != 0 || t.PercentBps != 0 {
			return fmt.Errorf("FREE 券金额字段必须全 0")
		}
	case marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_CUSTOM_MINOR:
		if t.AmountMinor <= 0 {
			return fmt.Errorf("CUSTOM_MINOR 券 amountMinor（发券面额上限）必须大于 0")
		}
		if t.PercentBps != 0 {
			return fmt.Errorf("CUSTOM_MINOR 券 percentBps 必须为 0")
		}
	case marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_DRAW_REWARD:
		if t.RewardTemplateId == 0 {
			return fmt.Errorf("DRAW_REWARD 券 rewardTemplateId 必须大于 0")
		}
		if t.RewardCount == 0 {
			return fmt.Errorf("DRAW_REWARD 券 rewardCount 必须大于 0")
		}
		if t.TemplateId != 0 && t.RewardTemplateId == t.TemplateId {
			return fmt.Errorf("DRAW_REWARD 券 rewardTemplateId 不能指向自身")
		}
	default:
		return fmt.Errorf("不支持的 CouponTemplateKind: %s", t.Kind)
	}
	return nil
}

// ValidateGrantCoupons 发券请求校验（count 上限 100；CUSTOM_MINOR 券须 ≤ 模板面额上限，
// 由 repo 读取模板后调用 CheckCustomAmount）。
func ValidateGrantCoupons(req *marketing.GrantCouponsRequest) error {
	if req.RequestId == "" || len(req.RequestId) > 64 {
		return fmt.Errorf("requestId 须非空且 ≤64 字符")
	}
	if req.TenantId == 0 {
		return fmt.Errorf("tenantId 必须大于 0")
	}
	if req.UserId == 0 {
		return fmt.Errorf("userId 必须大于 0")
	}
	if req.TemplateId == 0 {
		return fmt.Errorf("templateId 必须大于 0")
	}
	if req.Count == 0 || req.Count > maxGrantCoupons {
		return fmt.Errorf("count 须在 1..%d", maxGrantCoupons)
	}
	if req.CustomAmountMinor < 0 {
		return fmt.Errorf("customAmountMinor 不能为负")
	}
	switch req.Source {
	case marketing.CouponGrantSource_COUPON_GRANT_SOURCE_CONSOLE,
		marketing.CouponGrantSource_COUPON_GRANT_SOURCE_CAMPAIGN,
		marketing.CouponGrantSource_COUPON_GRANT_SOURCE_CHECKOUT_REWARD:
	default:
		return fmt.Errorf("不支持的 CouponGrantSource: %s", req.Source)
	}
	if req.Source == marketing.CouponGrantSource_COUPON_GRANT_SOURCE_CAMPAIGN && req.CampaignId == 0 {
		return fmt.Errorf("source=CAMPAIGN 时 campaignId 必须大于 0")
	}
	return nil
}

// CheckCustomAmount 发券实例面额校验（照抄 server_v2）：CUSTOM_MINOR 券须
// 0<custom≤模板 amountMinor（上限）；非 CUSTOM_MINOR 券 custom 必须 0。
func CheckCustomAmount(tpl *marketing.CouponTemplate, customAmountMinor int64) error {
	if tpl.Kind != marketing.CouponTemplateKind_COUPON_TEMPLATE_KIND_CUSTOM_MINOR {
		if customAmountMinor != 0 {
			return fmt.Errorf("非 CUSTOM_MINOR 券 customAmountMinor 必须为 0")
		}
		return nil
	}
	if customAmountMinor <= 0 {
		return fmt.Errorf("CUSTOM_MINOR 券须指定 customAmountMinor > 0")
	}
	if customAmountMinor > tpl.AmountMinor {
		return fmt.Errorf("customAmountMinor 不能大于模板面额上限 %d", tpl.AmountMinor)
	}
	return nil
}

// ExpiresAtUnixMillis 券到期时间（validityDays=0 → 0 无期限；否则 now + days）
func ExpiresAtUnixMillis(nowMillis int64, validityDays uint32) int64 {
	if validityDays == 0 {
		return 0
	}
	return nowMillis + int64(validityDays)*24*3600*1000
}
