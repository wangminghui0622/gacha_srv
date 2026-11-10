package gashapon

import (
	"context"

	baseClient "gacha_srv/app/base/baseClient"
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

type GashaponBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入）
	requestId string
}

func NewGashaponBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GashaponBuyLogic {
	return &GashaponBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *GashaponBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// GashaponBuy 扭蛋购买：网关发起 DTM SAGA 事务（7 分支，与 DigBuy/FullCarBuy 同构，统一 builder 见 saga/builder.go）。
//
//	[01 base.EvaluatePolicy scene=GASHAPON → 02 base.RiskEvaluate → 03 play.LockGashapon
//	 锁报价+participants/records 追加+余票-count → 04 money.AuthorizeWalletPayment 建单冻结
//	 → 05 play.SettleGashapon 抽 count 个 regular 奖+分阶段 LUCKY 判定+清空时终局
//	  LAST/MVP/LUCKY+落记录 → 06 play.GrantGashaponPrizes 发特别奖（永不 Aborted，空补偿）
//	 → 07 money.CaptureWalletPayment 捕获核销]
//	补偿：[空 → 空 → ReleaseGashapon 弹记录+余票+count+释放报价 → CancelPayment →
//	      SettleGashaponRevert 撤销记录/特别奖/sent/库存 → 空 → 空]
//
// 安全不变式：
//  1. userId 取 JWT 上下文，不接受客户端传入；
//  2. 单价取 play 服务端配置，LockGashapon 锁内重算 gross==单价×count 强校验
//     （6004 复用），客户端金额一律不信；
//  3. 特别奖判定确定性：seed=gid（SHA256 派生），同 gid 重放结果一致可核对。
//
// Submit 立即返回 pending，前端轮询 /gashapon/records?requestId= 直到出记录。
func (l *GashaponBuyLogic) GashaponBuy(req *types.GashaponBuyReq) (resp *types.GashaponBuyResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	if req.Count <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "抽数非法")
	}

	// ① gid = Idempotency-Key 头（重试场景复用）或服务端 uuid
	gid := uuid.New().String()
	if l.requestId != "" {
		if len(l.requestId) > 64 {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "幂等键过长（最长 64）")
		}
		gid = l.requestId
		// 幂等重放快路径：同 key 已有记录 → 直接 success（先于业务解析，M11b 教训）
		rec, qe := l.svcCtx.PlayRpc.ListGashaponRecords(l.ctx, &playClient.ListGashaponRecordsReq{
			RequestId: gid,
			UserId:    userId,
		})
		if qe == nil && rec != nil && len(rec.Items) > 0 {
			l.Infof("retry with same idempotency key, returning existing record, gid: %s", gid)
			return &types.GashaponBuyResp{RequestId: gid, Status: "success"}, nil
		}
	}

	// ② 单价取服务端（不信客户端）；总价=单价×count
	detail, err := l.svcCtx.PlayRpc.GetGashapon(l.ctx, &playClient.GetGashaponReq{ConfigId: req.ConfigId})
	if err != nil {
		l.Errorf("GashaponBuy GetGashapon failed, configId: %d, err: %v", req.ConfigId, err)
		return nil, errorx.FromError(err)
	}
	unitFen, err := utils.YuanToFen(detail.Price)
	if err != nil || unitFen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeAmountInvalid, "扭蛋价格无效")
	}
	grossFen := unitFen * req.Count

	// ③ 报价预计算：marketing 服务端算价（幂等 uk request_id=gid，重试复用同报价）
	quote, err := l.svcCtx.MarketingRpc.QuoteCheckout(l.ctx, &marketingClient.QuoteCheckoutReq{
		RequestId: gid,
		UserId:    userId,
		BoxId:     req.ConfigId,
		GrossFen:  grossFen,
	})
	if err != nil {
		l.Errorf("GashaponBuy QuoteCheckout failed, gid: %s, err: %v", gid, err)
		return nil, errorx.FromError(err)
	}

	c := l.svcCtx.Config
	b := saga.New(l.ctx, l.Logger, c.DtmServer.Target, gid).
		// 01 资格评估（基础域）：scene=GASHAPON
		Add(
			c.BaseRpcDirect+"/base.Base/EvaluatePolicy",
			c.BaseRpcDirect+"/base.Base/EvaluatePolicyRevert",
			&baseClient.EvaluatePolicyReq{
				RequestId: gid,
				UserId:    userId,
				BoxId:     req.ConfigId,
				Scene:     "GASHAPON",
			},
		).
		// 02 风控评估（基础域）
		Add(
			c.BaseRpcDirect+"/base.Base/RiskEvaluate",
			c.BaseRpcDirect+"/base.Base/RiskEvaluateRevert",
			&baseClient.RiskEvaluateReq{
				RequestId: gid,
				UserId:    userId,
				BoxId:     req.ConfigId,
				Scene:     "GASHAPON",
				AmountFen: grossFen,
			},
		).
		// 03 锁报价+参与记录+余票-count（玩法域）；补偿 ReleaseGashapon 弹记录+余票+count+释放报价
		Add(
			c.PlayRpcDirect+"/play.Play/LockGashapon",
			c.PlayRpcDirect+"/play.Play/ReleaseGashapon",
			&playClient.LockGashaponReq{
				RequestId:     gid,
				QuoteToken:    quote.QuoteToken,
				ConfigId:      req.ConfigId,
				UserId:        userId,
				Count:         req.Count,
				TotalPriceFen: grossFen,
			},
		).
		// 04 建单+冻结（资金域）
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
		// 05 抽 count 个 regular 奖+特别奖判定+落记录（玩法域）；补偿 SettleGashaponRevert 撤销记录/特别奖/sent/库存
		Add(
			c.PlayRpcDirect+"/play.Play/SettleGashapon",
			c.PlayRpcDirect+"/play.Play/SettleGashaponRevert",
			&playClient.SettleGashaponReq{
				RequestId: gid,
				UserId:    userId,
				ConfigId:  req.ConfigId,
				Count:     req.Count,
				Price:     utils.FenToYuan(quote.PayableFen),
			},
		).
		// 06 发特别奖（玩法域）：永不 Aborted、空补偿（奖品先于捕获且不追回）
		Add(
			c.PlayRpcDirect+"/play.Play/GrantGashaponPrizes",
			c.PlayRpcDirect+"/play.Play/GrantGashaponPrizesRevert",
			&playClient.GrantGashaponPrizesReq{
				RequestId: gid,
				UserId:    userId,
				ConfigId:  req.ConfigId,
			},
		).
		// 07 捕获核销（资金域末步）：腿 POSTED、核销报价（CommitQuote）、订单 PAID
		Add(
			c.MoneyRpcDirect+"/money.Money/CaptureWalletPayment",
			c.MoneyRpcDirect+"/money.Money/CaptureWalletPaymentRevert",
			&moneyClient.CaptureWalletPaymentReq{
				RequestId:  gid,
				QuoteToken: quote.QuoteToken,
			},
		)
	// 分支错误语义：Aborted=业务失败→补偿；Internal=未知→退避重试（builder 默认 RetryLimit=10）。

	// Submit 立即返回：只代表事务被 DTM 接受，抽奖开奖异步执行。
	// 幂等回查：同 gid 已存在（重试成功）→ 直接 success；未命中才是真故障。
	var recheck saga.Recheck
	if l.requestId != "" {
		recheck = func(fallbackCtx context.Context) error {
			_, qe := l.svcCtx.PlayRpc.ListGashaponRecords(fallbackCtx, &playClient.ListGashaponRecordsReq{
				RequestId: gid,
				UserId:    userId,
			})
			return qe
		}
	}
	alreadyDone, err := b.Submit("扭蛋请求提交失败，请稍后重试", recheck)
	if err != nil {
		return nil, err
	}
	if alreadyDone {
		return &types.GashaponBuyResp{RequestId: gid, Status: "success"}, nil
	}
	l.Infof("dtm saga submitted, gid: %s, configId: %d, userId: %d, count: %d",
		gid, req.ConfigId, userId, req.Count)
	return &types.GashaponBuyResp{RequestId: gid, Status: "pending"}, nil
}
