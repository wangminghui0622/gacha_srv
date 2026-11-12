package draw

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	moneyClient "gacha_srv/app/money/moneyClient"
	"gacha_srv/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

// SubmitDrawSaga 同步预检（资格+风控）后提交 5 分支抽奖 SAGA
// （BoxBuy 与 CampaignBoxOpen 共用，统一 builder 见 saga/builder.go）：
//
//	[01 marketing.LockQuote 锁券/资格 → 02 money.AuthorizeWalletPayment 建单+冻结 →
//	 03 draw.SettleDraw 开奖 → 04 money.GrantAsset 发资产 →
//	 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseQuote → CancelPayment → SettleDrawRevert → GrantAssetRevert → 空]
//
// 资格评估（base.EvaluatePolicy）与风控评估（base.RiskEvaluate）为「闸门」而非「事务」，
// 已抽离为同步预检（saga.Precheck）：拒绝时立即返回业务码 4001/4002，前端当场可见，
// 免去 DTM 编排与「pending 后无记录」的静默失败。
//
// quote 为已试算的报价（券/积分/活动宝箱资格全抵均在 QuoteCheckout 完成，幂等 uk request_id=gid），
// 02/03 全用报价实付值；SettleDraw price=FenToYuan(payable)（资格全抵时自然为 "0.00"）。
// Submit 失败且带幂等键（requestId 非空）→ 回查抽赏记录：命中=同 gid 事务已存在（重试成功），
// 返回 alreadyDone=true；未命中才是真故障。
// 分支错误语义：Aborted=业务失败→补偿；Internal=未知→退避重试（RetryLimit=10 封顶）。
func SubmitDrawSaga(ctx context.Context, svcCtx *svc.ServiceContext, log logx.Logger, gid, requestId string, userId, boxId, amountFen int64, quote *marketingClient.QuoteCheckoutResp) (alreadyDone bool, err error) {
	// 同步预检：资格评估 + 风控评估（拒绝立即返回，不入分布式事务）
	if err := saga.Precheck(ctx, log, svcCtx.BaseRpc, gid, userId, boxId, amountFen, "DRAW"); err != nil {
		return false, err
	}

	c := svcCtx.Config
	b := saga.New(ctx, log, c.DtmServer.Target, gid).
		// 01 锁报价（营销域）：报价 OPEN→LOCKED、券/资格 AVAILABLE→LOCKED；补偿 ReleaseQuote 释放
		Add(
			c.MarketingRpcDirect+"/marketing.Marketing/LockQuote",
			c.MarketingRpcDirect+"/marketing.Marketing/ReleaseQuote",
			&marketingClient.LockQuoteReq{
				RequestId:  gid,
				QuoteToken: quote.QuoteToken,
			},
		).
		// 02 建单+冻结（资金域）：gross=报价实付，cash/loyalty 按报价拆腿
		AddBranch(saga.WalletAuthorizeBranch(c.MoneyRpcDirect, gid, userId, boxId, quote)).
		// 03 开奖（抽奖域）：price 传报价实付，SettleDraw 落记录
		Add(
			c.DrawRpcDirect+"/draw.Draw/SettleDraw",
			c.DrawRpcDirect+"/draw.Draw/SettleDrawRevert",
			&drawClient.SettleDrawReq{
				BoxId:              boxId,
				UserId:             userId,
				RequestId:          gid,
				Price:              utils.FenToYuan(quote.PayableFen),
				LoyaltyPointsDebit: quote.LoyaltyPointsDebit,
			},
		).
		// 04 发资产（资金域）：asset_no="a_"+requestId 幂等，奖品名从开奖记录反查
		Add(
			c.MoneyRpcDirect+"/money.Money/GrantAsset",
			c.MoneyRpcDirect+"/money.Money/GrantAssetRevert",
			&moneyClient.GrantAssetReq{
				UserId:    userId,
				BoxId:     boxId,
				RequestId: gid,
			},
		).
		// 05 捕获核销（资金域末步）：腿 POSTED、核销报价（CommitQuote）、订单 PAID
		AddBranch(saga.WalletCaptureBranch(c.MoneyRpcDirect, gid, quote))

	// Submit 立即返回：只代表事务被 DTM 接受，开奖异步执行。
	// 幂等回查：同 gid 已存在（重试成功）→ alreadyDone=true；未命中才是真故障。
	var recheck saga.Recheck
	if requestId != "" {
		recheck = func(fallbackCtx context.Context) error {
			_, qe := svcCtx.DrawRpc.GetRecord(fallbackCtx, &drawClient.GetRecordReq{
				RequestId: gid,
				UserId:    userId,
			})
			return qe
		}
	}
	return b.Submit("开奖请求提交失败，请稍后重试", recheck)
}
