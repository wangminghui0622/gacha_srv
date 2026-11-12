package combochain

import (
	"context"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	moneyClient "gacha_srv/app/money/moneyClient"
	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type ComboChainBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入）
	requestId string
}

func NewComboChainBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComboChainBuyLogic {
	return &ComboChainBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *ComboChainBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// ComboChainBuy 连击链购票：网关发起 DTM SAGA 事务（同步预检资格+风控后 5 分支，与 DigBuy/GashaponBuy 同构，统一 builder 见 saga/builder.go）。
//
//	[01 play.LockComboChain 锁报价+participants 追加+余票-1
//	 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleComboChain 落记录+末票时整车结算（连击档触发/集中奖/全局槽分配/
//	  连击累计）→ 04 play.GrantComboChainPrizes 发结算奖（永不 Aborted，空补偿）
//	 → 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseComboChain 弹参与+余票+1+释放报价 → CancelPayment →
//	      SettleComboChainRevert 撤销记录/结算奖/连击快照/档位剩余 → 空 → 空]
//	同步预检：base.EvaluatePolicy（scene=COMBO_CHAIN）+ base.RiskEvaluate，拒绝立即返回 4001/4002。
//
// 安全不变式：
//  1. userId 取 JWT 上下文，不接受客户端传入；
//  2. 单价取 play 服务端配置，LockComboChain 锁内重算 gross==单价 强校验
//     （6004 复用），客户端金额一律不信；
//  3. 整车结算确定性：seed=gid（SHA256 派生），同 gid 重放结果一致可核对。
//
// Submit 立即返回 pending，前端轮询 /combo-chain/records?requestId= 直到出记录。
func (l *ComboChainBuyLogic) ComboChainBuy(req *types.ComboChainBuyReq) (resp *types.ComboChainBuyResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}

	// ① gid = Idempotency-Key 头（重试场景复用）或服务端 uuid
	gid := uuid.New().String()
	if l.requestId != "" {
		if len(l.requestId) > 64 {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "幂等键过长（最长 64）")
		}
		gid = l.requestId
		// 幂等重放快路径：同 key 已有记录 → 直接 success（先于业务解析）
		rec, qe := l.svcCtx.PlayRpc.ListComboChainRecords(l.ctx, &playClient.ListComboChainRecordsReq{
			RequestId: gid,
			UserId:    userId,
		})
		if qe == nil && rec != nil && len(rec.Items) > 0 {
			l.Infof("retry with same idempotency key, returning existing record, gid: %s", gid)
			return &types.ComboChainBuyResp{RequestId: gid, Status: "success"}, nil
		}
	}

	// ② 单价取服务端（不信客户端）
	detail, err := l.svcCtx.PlayRpc.GetComboChain(l.ctx, &playClient.GetComboChainReq{ConfigId: req.ConfigId, UserId: userId})
	if err != nil {
		l.Errorf("ComboChainBuy GetComboChain failed, configId: %d, err: %v", req.ConfigId, err)
		return nil, errorx.FromError(err)
	}
	unitFen, err := utils.YuanToFen(detail.Price)
	if err != nil || unitFen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeAmountInvalid, "连击链价格无效")
	}

	// ③ 报价预计算：marketing 服务端算价（幂等 uk request_id=gid，重试复用同报价）
	quote, err := l.svcCtx.MarketingRpc.QuoteCheckout(l.ctx, &marketingClient.QuoteCheckoutReq{
		RequestId: gid,
		UserId:    userId,
		BoxId:     req.ConfigId,
		GrossFen:  unitFen,
	})
	if err != nil {
		l.Errorf("ComboChainBuy QuoteCheckout failed, gid: %s, err: %v", gid, err)
		return nil, errorx.FromError(err)
	}

	// 同步预检：资格评估 + 风控评估（拒绝立即返回，不入分布式事务）
	if err := saga.Precheck(l.ctx, l.Logger, l.svcCtx.BaseRpc, gid, userId, req.ConfigId, unitFen, "COMBO_CHAIN"); err != nil {
		return nil, err
	}

	c := l.svcCtx.Config
	b := saga.New(l.ctx, l.Logger, c.DtmServer.Target, gid).
		// 01 锁报价+参与追加+余票-1（玩法域）；补偿 ReleaseComboChain 弹参与+余票+1+释放报价
		Add(
			c.PlayRpcDirect+"/play.Play/LockComboChain",
			c.PlayRpcDirect+"/play.Play/ReleaseComboChain",
			&playClient.LockComboChainReq{
				RequestId:     gid,
				QuoteToken:    quote.QuoteToken,
				ConfigId:      req.ConfigId,
				UserId:        userId,
				TotalPriceFen: unitFen,
			},
		).
		// 02 建单+冻结（资金域）
		Add(
			c.MoneyRpcDirect+"/money.Money/AuthorizeWalletPayment",
			c.MoneyRpcDirect+"/money.Money/CancelPayment",
			&moneyClient.AuthorizeWalletPaymentReq{
				RequestId:          gid,
				UserId:             userId,
				BoxId:              req.ConfigId,
				GrossAmount:        utils.FenToYuan(quote.PayableFen),
				CashAmount:         utils.FenToYuan(quote.CashFen),
				LoyaltyAmount:      "0.00",
				LoyaltyPointsDebit: 0,
			},
		).
		// 03 落记录+末票整车结算（玩法域）；补偿 SettleComboChainRevert 撤销记录/结算奖/连击快照/档位剩余
		Add(
			c.PlayRpcDirect+"/play.Play/SettleComboChain",
			c.PlayRpcDirect+"/play.Play/SettleComboChainRevert",
			&playClient.SettleComboChainReq{
				RequestId: gid,
				UserId:    userId,
				ConfigId:  req.ConfigId,
				Price:     utils.FenToYuan(quote.PayableFen),
			},
		).
		// 04 发结算奖（玩法域）：永不 Aborted、空补偿（奖品先于捕获且不追回）
		Add(
			c.PlayRpcDirect+"/play.Play/GrantComboChainPrizes",
			c.PlayRpcDirect+"/play.Play/GrantComboChainPrizesRevert",
			&playClient.GrantComboChainPrizesReq{
				RequestId: gid,
				UserId:    userId,
				ConfigId:  req.ConfigId,
			},
		).
		// 05 捕获核销（资金域末步）：腿 POSTED、核销报价（CommitQuote）、订单 PAID
		Add(
			c.MoneyRpcDirect+"/money.Money/CaptureWalletPayment",
			c.MoneyRpcDirect+"/money.Money/CaptureWalletPaymentRevert",
			&moneyClient.CaptureWalletPaymentReq{
				RequestId:  gid,
				QuoteToken: quote.QuoteToken,
			},
		)
	// 分支错误语义：Aborted=业务失败→补偿；Internal=未知→退避重试（builder 默认 RetryLimit=10）。

	// Submit 立即返回：只代表事务被 DTM 接受，开奖异步执行。
	// 幂等回查：同 gid 已存在（重试成功）→ 直接 success；未命中才是真故障。
	var recheck saga.Recheck
	if l.requestId != "" {
		recheck = func(fallbackCtx context.Context) error {
			_, qe := l.svcCtx.PlayRpc.ListComboChainRecords(fallbackCtx, &playClient.ListComboChainRecordsReq{
				RequestId: gid,
				UserId:    userId,
			})
			return qe
		}
	}
	alreadyDone, err := b.Submit("连击链请求提交失败，请稍后重试", recheck)
	if err != nil {
		return nil, err
	}
	if alreadyDone {
		return &types.ComboChainBuyResp{RequestId: gid, Status: "success"}, nil
	}
	l.Infof("dtm saga submitted, gid: %s, configId: %d, userId: %d", gid, req.ConfigId, userId)
	return &types.ComboChainBuyResp{RequestId: gid, Status: "pending"}, nil
}
