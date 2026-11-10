package logic

import (
	"gacha_srv/app/marketing/marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
)

// 报价锁状态（quote_locks.status）：1 OPEN 2 LOCKED 3 USED 4 RELEASED 5 EXPIRED
const (
	quoteOpen     = 1
	quoteLocked   = 2
	quoteUsed     = 3
	quoteReleased = 4
	quoteExpired  = 5
)

// 用户券状态（user_coupons.status）：1 AVAILABLE 2 LOCKED 3 USED 4 EXPIRED
const (
	couponAvailable = 1
	couponLocked    = 2
	couponUsed      = 3
	couponExpired   = 4
)

// 券模板状态（coupon_templates.status）：0 草稿 1 已发布 2 已停用
const templatePublished = 1

// quoteStatusName 状态码 → 展示名（响应 Status 字段与测试断言用）
func quoteStatusName(status int64) string {
	switch status {
	case quoteOpen:
		return "OPEN"
	case quoteLocked:
		return "LOCKED"
	case quoteUsed:
		return "USED"
	case quoteReleased:
		return "RELEASED"
	default:
		return "EXPIRED"
	}
}

// minInt64 取较小值（本机 go 版本兼容写法，不依赖内置 min）
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// buildQuoteResp 由报价锁行组装响应（幂等读回路径与新建路径共用）
func buildQuoteResp(q *gamemodelnc.QuoteLocks) *marketing.QuoteCheckoutResp {
	return &marketing.QuoteCheckoutResp{
		QuoteToken:         q.QuoteToken,
		GrossFen:           q.GrossFen,
		CouponDiscountFen:  q.CouponDiscountFen,
		CouponNo:           q.CouponNo,
		LoyaltyPointsDebit: q.LoyaltyPointsDebit,
		LoyaltyFen:         q.LoyaltyFen,
		PayableFen:         q.PayableFen,
		CashFen:            q.CashFen,
	}
}
