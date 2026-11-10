package combochain

import (
	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/types"
)

// prizeItemToView play.PrizeItem → 网关 DigPrizeItem 视图映射
func prizeItemToView(p *playClient.PrizeItem) types.DigPrizeItem {
	if p == nil {
		return types.DigPrizeItem{}
	}
	return types.DigPrizeItem{
		Kind:             p.Kind,
		Amount:           p.Amount,
		Points:           p.Points,
		CouponTemplateNo: p.CouponTemplateNo,
		CouponName:       p.CouponName,
	}
}
