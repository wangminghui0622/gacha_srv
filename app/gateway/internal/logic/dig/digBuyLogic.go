package dig

import (
	"context"
	"fmt"

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

type DigBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入）
	requestId string
}

func NewDigBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DigBuyLogic {
	return &DigBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *DigBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// DigBuy 挖宝购买：网关发起 DTM SAGA 事务（同步预检资格+风控后 5 分支，与 BoxBuy 同构，统一 builder 见 saga/builder.go）。
//
//	[01 play.LockDigBox 锁报价+会话位图 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleDigBox 揭晓+落记录 → 04 play.GrantDigPrizes 发奖（永不 Aborted，空补偿）
//	 → 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseDigBox 清位回滚+释放报价 → CancelPayment →
//	      SettleDigBoxRevert 撤销记录 → 空 → 空]
//	同步预检：base.EvaluatePolicy（scene=DIG_BOX）+ base.RiskEvaluate，拒绝立即返回 4001/4002。
//
// 安全不变式：
//  1. userId 取 JWT 上下文，不接受客户端传入；
//  2. 单价取 play 服务端配置，格子数网关解析（takeAll=会话空位），
//     LockDigBox 锁内重算 gross==单价×格数 强校验（6018），客户端金额一律不信；
//  3. 会话按 (user,config) 唯一：满格 COMPLETED 在下次购买时回收（D5）。
//
// Submit 立即返回 pending，前端轮询 /dig-boxes/records?requestId= 直到出记录。
func (l *DigBuyLogic) DigBuy(req *types.DigBuyReq) (resp *types.DigBuyResp, err error) {
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
		// 幂等重放快路径：同 key 已有记录 → 直接 success。
		// 必须在格子解析之前——全开后重放时箱已挖完，resolveDigCells 会先报
		// 6002「已挖完」，永远到不了 DTM 的重复 gid 兜底。
		rec, qe := l.svcCtx.PlayRpc.ListDigRecords(l.ctx, &playClient.ListDigRecordsReq{
			RequestId: gid,
			UserId:    userId,
		})
		if qe == nil && rec != nil && len(rec.Items) > 0 {
			l.Infof("retry with same idempotency key, returning existing record, gid: %s", gid)
			return &types.DigBuyResp{RequestId: gid, Status: "success"}, nil
		}
	}

	// ② 单价取服务端（不信客户端）；gross = 格数 × 单价
	var unitPrice string
	boxes, err := l.svcCtx.PlayRpc.ListDigBoxes(l.ctx, &playClient.ListDigBoxesReq{})
	if err != nil {
		l.Errorf("DigBuy ListDigBoxes failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	for _, it := range boxes.Items {
		if it.BoxId == req.BoxId {
			unitPrice = it.Price
			break
		}
	}
	if unitPrice == "" {
		return nil, errorx.NewCodeError(errorx.CodeDigBoxNotFound, "挖宝箱不存在")
	}
	unitFen, err := utils.YuanToFen(unitPrice)
	if err != nil || unitFen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeAmountInvalid, "挖宝价格无效")
	}

	// ③ 格子解析（takeAll → 会话空位显式化，补偿载荷由此精确）
	cells, err := resolveDigCells(l.ctx, l.svcCtx, userId, req.BoxId, req.CellIndexes, req.TakeAll)
	if err != nil {
		return nil, err
	}
	grossFen := int64(len(cells)) * unitFen
	sessionNo := fmt.Sprintf("dg_%d_%d", userId, req.BoxId)

	// ④ 报价预计算：marketing 服务端算价（幂等 uk request_id=gid，重试复用同报价）；
	//    挖宝不用券/积分抵扣，payable=gross
	quote, err := l.svcCtx.MarketingRpc.QuoteCheckout(l.ctx, &marketingClient.QuoteCheckoutReq{
		RequestId: gid,
		UserId:    userId,
		BoxId:     req.BoxId,
		GrossFen:  grossFen,
	})
	if err != nil {
		l.Errorf("DigBuy QuoteCheckout failed, gid: %s, err: %v", gid, err)
		return nil, errorx.FromError(err)
	}

	// 同步预检：资格评估 + 风控评估（拒绝立即返回，不入分布式事务）
	if err := saga.Precheck(l.ctx, l.Logger, l.svcCtx.BaseRpc, gid, userId, req.BoxId, grossFen, "DIG_BOX"); err != nil {
		return nil, err
	}

	c := l.svcCtx.Config
	b := saga.New(l.ctx, l.Logger, c.DtmServer.Target, gid).
		// 01 锁报价+会话位图（玩法域）；补偿 ReleaseDigBox 清位回滚+释放报价
		Add(
			c.PlayRpcDirect+"/play.Play/LockDigBox",
			c.PlayRpcDirect+"/play.Play/ReleaseDigBox",
			&playClient.LockDigBoxReq{
				RequestId:     gid,
				QuoteToken:    quote.QuoteToken,
				ConfigId:      req.BoxId,
				SessionNo:     sessionNo,
				CellIndexes:   cells,
				TakeAll:       false, // 网关已把 takeAll 解析为显式格子（补偿精确回滚依据）
				TotalPriceFen: grossFen,
			},
		).
		// 02 建单+冻结（资金域）：挖宝无券/积分，gross=cash=实付
		Add(
			c.MoneyRpcDirect+"/money.Money/AuthorizeWalletPayment",
			c.MoneyRpcDirect+"/money.Money/CancelPayment",
			&moneyClient.AuthorizeWalletPaymentReq{
				RequestId:          gid,
				UserId:             userId,
				BoxId:              req.BoxId,
				GrossAmount:        utils.FenToYuan(quote.PayableFen),
				CashAmount:         utils.FenToYuan(quote.CashFen),
				LoyaltyAmount:      "0.00",
				LoyaltyPointsDebit: 0,
			},
		).
		// 03 揭晓+落记录（玩法域）；补偿 SettleDigBoxRevert 撤销记录
		Add(
			c.PlayRpcDirect+"/play.Play/SettleDigBox",
			c.PlayRpcDirect+"/play.Play/SettleDigBoxRevert",
			&playClient.SettleDigBoxReq{
				RequestId:   gid,
				UserId:      userId,
				ConfigId:    req.BoxId,
				SessionNo:   sessionNo,
				CellIndexes: cells,
				TakeAll:     false,
				Price:       utils.FenToYuan(quote.PayableFen),
			},
		).
		// 04 发奖（玩法域）：永不 Aborted、空补偿（奖品先于捕获且不追回）
		Add(
			c.PlayRpcDirect+"/play.Play/GrantDigPrizes",
			c.PlayRpcDirect+"/play.Play/GrantDigPrizesRevert",
			&playClient.GrantDigPrizesReq{
				RequestId: gid,
				UserId:    userId,
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

	// Submit 立即返回：只代表事务被 DTM 接受，开格异步执行。
	// 幂等回查：同 gid 已存在（重试成功）→ 直接 success；未命中才是真故障。
	var recheck saga.Recheck
	if l.requestId != "" {
		recheck = func(fallbackCtx context.Context) error {
			_, qe := l.svcCtx.PlayRpc.ListDigRecords(fallbackCtx, &playClient.ListDigRecordsReq{
				RequestId: gid,
				UserId:    userId,
			})
			return qe
		}
	}
	alreadyDone, err := b.Submit("挖宝请求提交失败，请稍后重试", recheck)
	if err != nil {
		return nil, err
	}
	if alreadyDone {
		return &types.DigBuyResp{RequestId: gid, Status: "success"}, nil
	}
	l.Infof("dtm saga submitted, gid: %s, boxId: %d, userId: %d, cells: %d",
		gid, req.BoxId, userId, len(cells))
	return &types.DigBuyResp{RequestId: gid, Status: "pending"}, nil
}
