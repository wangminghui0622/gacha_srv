package memorymatch

import (
	"context"
	"strings"
	"time"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	moneyClient "gacha_srv/app/money/moneyClient"
	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/task/task"
	"gacha_srv/app/task/taskClient"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type MemoryMatchBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入，强制非空）
	requestId string
}

func NewMemoryMatchBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext, requestId string) *MemoryMatchBuyLogic {
	return &MemoryMatchBuyLogic{
		Logger:    logx.WithContext(ctx),
		ctx:       ctx,
		svcCtx:    svcCtx,
		requestId: requestId,
	}
}

// pollSession 轮询 SAGA 会话建立（100ms × 100 = 10s）；返回 (resp, found, expired)
func (l *MemoryMatchBuyLogic) pollSession(ctx context.Context, gid string, userId int64) (*playClient.GetMemoryMatchSessionByRequestResp, bool, bool) {
	for i := 0; i < 100; i++ {
		sess, err := l.svcCtx.PlayRpc.GetMemoryMatchSessionByRequest(ctx, &playClient.GetMemoryMatchSessionByRequestReq{
			RequestId: gid,
			UserId:    userId,
		})
		if err == nil && sess != nil {
			switch int64(sess.State) {
			case 1: // PENDING：SAGA 仍在途，继续等
			case 7: // EXPIRED：事务已补偿，购买失败
				return sess, true, true
			default: // AWAITING_DESTINY/AWAITING_SECOND/PLAYING/COMPLETED
				return sess, true, false
			}
		}
		select {
		case <-ctx.Done():
			return nil, false, false
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil, false, false
}

func (l *MemoryMatchBuyLogic) successResp(sess *playClient.GetMemoryMatchSessionByRequestResp) *types.MemoryMatchPurchaseResp {
	return &types.MemoryMatchPurchaseResp{
		SessionId:       sess.SessionId,
		PurchasedRounds: sess.PurchasedRounds,
		PaidMinor:       sess.PaidMinor,
		State:           int64(sess.State),
		FacePresetId:    sess.FacePresetId,
		BackPresetId:    sess.BackPresetId,
	}
}

// MemoryMatchBuy 记忆翻牌购买：网关发起 DTM SAGA 事务（同步预检资格+风控后 5 分支，与 FullCarBuy 同构，统一 builder 见 saga/builder.go）。
//
//	[01 play.LockMemoryMatch 锁报价+会话插入 PENDING+配置/外观/权益/价格校验
//	 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleMemoryMatch PENDING→AWAITING_DESTINY（engine_seed+擂主分成）
//	 → 04 play.GrantMemoryMatchPrizes 空操作（购买时无奖）→ 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[LockMemoryMatchRevert 会话 EXPIRED+释放报价 → CancelPayment →
//	      SettleMemoryMatchRevert 回 PENDING+清种子+回扣分成 → 空 → 空]
//	同步预检：base.EvaluatePolicy（scene=MEMORY_MATCH）+ base.RiskEvaluate，拒绝立即返回 4001/4002。
//
// 安全不变式：
//  1. userId 取 JWT 上下文，不接受客户端传入；
//  2. 单价取 play 服务端配置，LockMemoryMatch 锁内重算 gross==配置价 强校验，客户端金额一律不信；
//  3. 外卡 WECHAT/ALIPAY 不支持 → 6021（仅 WALLET，偏离 2）；
//  4. 响应同步返回会话（老项目同步购买同款形状）：Submit 后按 requestId 轮询会话建立（≤10s）。
func (l *MemoryMatchBuyLogic) MemoryMatchBuy(req *types.MemoryMatchPurchaseReq) (resp *types.MemoryMatchPurchaseResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}

	// ① 幂等键强制（D3）；gid = Idempotency-Key
	if l.requestId == "" || len(l.requestId) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "Idempotency-Key 头必填（最长 64）")
	}
	gid := l.requestId

	// ② 幂等重放快路径：同 key 已有会话 → 直接返回（先于业务解析，M11b 教训；
	// 与 FullCarBuy 快路径同构：单次查询不烧请求预算——网关 rest 超时 3s，
	// 轮询空会话会把预算耗尽；PENDING=并发在途，交由 Submit 同 gid 冲突回查兜底）
	if sess, err := l.svcCtx.PlayRpc.GetMemoryMatchSessionByRequest(l.ctx, &playClient.GetMemoryMatchSessionByRequestReq{
		RequestId: gid,
		UserId:    userId,
	}); err == nil && sess != nil && int64(sess.State) != 1 {
		if int64(sess.State) == 7 { // EXPIRED：事务已补偿
			l.Errorf("MemoryMatchBuy idempotent replay hit expired session, gid: %s", gid)
			return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "该幂等键对应购买已撤销，请更换幂等键重试")
		}
		l.Infof("MemoryMatchBuy idempotent replay, returning existing session, gid: %s, sessionId: %d", gid, sess.SessionId)
		l.applyTaskProgress(userId, gid, sess.PaidMinor)
		return l.successResp(sess), nil
	}

	// ③ 支付方式：仅 WALLET（偏离 2，外卡 6021）
	method := strings.ToUpper(strings.TrimSpace(req.PaymentMethod))
	if method == "" {
		method = "WALLET"
	}
	if method != "WALLET" {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchPaymentUnsupported, "支付方式暂不支持，仅支持钱包支付")
	}

	// ④ 单价取服务端（不信客户端）；face/back 预设存在性与权益由 LockMemoryMatch 锁内校验
	detail, err := l.svcCtx.PlayRpc.GetMemoryMatchConfig(l.ctx, &playClient.GetMemoryMatchConfigReq{
		ConfigId: req.ConfigId,
		UserId:   userId,
	})
	if err != nil {
		l.Errorf("MemoryMatchBuy GetMemoryMatchConfig failed, configId: %d, err: %v", req.ConfigId, err)
		return nil, errorx.FromError(err)
	}
	unitFen, err := utils.YuanToFen(detail.Config.PurchasePrice)
	if err != nil || unitFen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeAmountInvalid, "记忆翻牌价格无效")
	}

	// ⑤ 报价预计算：marketing 服务端算价（幂等 uk request_id=gid，重试复用同报价；券抵扣在 payable/cash 体现）
	quote, err := l.svcCtx.MarketingRpc.QuoteCheckout(l.ctx, &marketingClient.QuoteCheckoutReq{
		RequestId:    gid,
		UserId:       userId,
		BoxId:        req.ConfigId,
		UserCouponId: req.UserCouponId,
		GrossFen:     unitFen,
	})
	if err != nil {
		l.Errorf("MemoryMatchBuy QuoteCheckout failed, gid: %s, err: %v", gid, err)
		return nil, errorx.FromError(err)
	}

	// 同步预检：资格评估 + 风控评估（拒绝立即返回，不入分布式事务）
	if err := saga.Precheck(l.ctx, l.Logger, l.svcCtx.BaseRpc, gid, userId, req.ConfigId, unitFen, "MEMORY_MATCH"); err != nil {
		return nil, err
	}

	c := l.svcCtx.Config
	b := saga.New(l.ctx, l.Logger, c.DtmServer.Target, gid).
		// 01 锁报价+会话插入 PENDING（玩法域）；补偿 LockMemoryMatchRevert 会话 EXPIRED+释放报价
		Add(
			c.PlayRpcDirect+"/play.Play/LockMemoryMatch",
			c.PlayRpcDirect+"/play.Play/LockMemoryMatchRevert",
			&playClient.LockMemoryMatchReq{
				RequestId:     gid,
				QuoteToken:    quote.QuoteToken,
				ConfigId:      req.ConfigId,
				UserId:        userId,
				FacePresetId:  req.FacePresetId,
				BackPresetId:  req.BackPresetId,
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
		// 03 PENDING→AWAITING_DESTINY（玩法域；sessionId 由会话 uk request_id 推导，提交时未知传 0）
		Add(
			c.PlayRpcDirect+"/play.Play/SettleMemoryMatch",
			c.PlayRpcDirect+"/play.Play/SettleMemoryMatchRevert",
			&playClient.SettleMemoryMatchReq{
				RequestId: gid,
				UserId:    userId,
				ConfigId:  req.ConfigId,
				SessionId: 0,
				Price:     utils.FenToYuan(quote.PayableFen),
			},
		).
		// 04 发奖空操作（玩法域）：购买时无奖，终局结算同步发奖；永不 Aborted、空补偿
		Add(
			c.PlayRpcDirect+"/play.Play/GrantMemoryMatchPrizes",
			c.PlayRpcDirect+"/play.Play/GrantMemoryMatchPrizesRevert",
			&playClient.GrantMemoryMatchPrizesReq{
				RequestId: gid,
				UserId:    userId,
				SessionId: 0,
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

	// Submit 失败很可能是「同 gid 事务已存在」：回查会话。
	// 响应需完整会话（sessionId/state/外观），故 recheck 不委托 builder、手动轮询兜底。
	if _, err := b.Submit("记忆翻牌请求提交失败，请稍后重试", nil); err != nil {
		fallbackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if sess, found, _ := l.pollSession(fallbackCtx, gid, userId); found {
			l.Infof("MemoryMatchBuy submit failed but session exists, gid: %s, sessionId: %d", gid, sess.SessionId)
			l.applyTaskProgress(userId, gid, quote.PayableFen)
			return l.successResp(sess), nil
		}
		l.Errorf("dtm saga submit failed, gid: %s, err: %v", gid, err)
		return nil, err
	}
	l.Infof("dtm saga submitted, gid: %s, configId: %d, userId: %d", gid, req.ConfigId, userId)

	// ⑥ 轮询会话建立（≤10s）：老项目同步购买同款响应形状（sessionId/state/appearance）
	pollCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if sess, found, expired := l.pollSession(pollCtx, gid, userId); found {
		if expired {
			l.Errorf("MemoryMatchBuy saga compensated, gid: %s", gid)
			return nil, errorx.NewCodeError(errorx.CodeDTMError, "购买未完成（事务已回滚），请更换幂等键重试")
		}
		// 购买挂钩（D9）：实付金额记入 DRAW_SPEND_MINOR；失败仅记日志不阻断购买
		l.applyTaskProgress(userId, gid, quote.PayableFen)
		return l.successResp(sess), nil
	}
	l.Errorf("MemoryMatchBuy poll timeout, gid: %s", gid)
	return nil, errorx.NewCodeError(errorx.CodeDTMError, "购买处理中，请稍后以相同幂等键重试")
}

// applyTaskProgress 购买挂钩（D9）：事件号="pay:"+订单号（gid），metric=DRAW_SPEND_MINOR，
// amount=实付（分）。INSERT IGNORE 事件幂等；失败仅记日志（老项目事件消费失败同样重试不阻断）。
func (l *MemoryMatchBuyLogic) applyTaskProgress(userId int64, gid string, paidMinor int64) {
	if paidMinor <= 0 {
		return
	}
	if _, err := l.svcCtx.TaskRpc.ApplyTaskProgress(l.ctx, &taskClient.ApplyTaskProgressReq{
		UserId:  userId,
		EventId: "pay:" + gid,
		Metric:  task.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR,
		Amount:  paidMinor,
	}); err != nil {
		l.Errorf("applyTaskProgress failed (non-blocking), gid: %s, userId: %d, err: %v", gid, userId, err)
	}
}
