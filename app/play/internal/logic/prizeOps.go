package logic

import (
	"context"

	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/svc"
)

// ══════════════ 玩法发奖公共常量 ═══════════════
// 发奖类型与 wallet_kind 对齐（旧仓 campaignOps 定义，玩法域共用拆出）。

// 奖品类型（prizes_json 元素 kind / grant 分发键）
const (
	prizeCash   int64 = 1 // 现金（CreditWallet kind=1）
	prizePoints int64 = 2 // 积分（CreditWallet kind=4）
	prizeCoupon int64 = 3 // 券（"uc_"+grantNo 确定性单号）
)

// 钱包类型（money.CreditWallet kind）
const (
	walletCash         int32 = 1
	walletLoyaltyPoint int32 = 4
)

// couponNameOf 经营销服务查券名（展示用；NotFound/系统错误静默降级返回空——旧仓同款容错）
func couponNameOf(ctx context.Context, svcCtx *svc.ServiceContext, templateNo string) string {
	if templateNo == "" {
		return ""
	}
	tpl, err := svcCtx.MarketingRpc.GetCouponTemplateByNo(ctx, &marketingClient.GetCouponTemplateByNoReq{TemplateNo: templateNo})
	if err != nil {
		return ""
	}
	return tpl.GetName()
}
