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

// MemoryMatchBuy 记忆翻牌购买：网关发起 DTM SAGA 事务（同步预检资格+风控后 5 分支）。
//
//	[01 play.LockMemoryMatch 锁报价+会话插入 PENDING+配置/外观/权益/价格校验
//	 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleMemoryMatch PENDING→AWAITING_DESTINY（engine_seed+擂主分成）
//	 → 04 play.GrantMemoryMatchPrizes 空操作（购买时无奖）→ 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[LockMemoryMatchRevert 会话 EXPIRED+释放报价 → CancelPayment →
//	      SettleMemoryMatchRevert 回 PENDING+清种子+回扣分成 → 空 → 空]
//
// 与其它玩法不同：提交后按 requestId 轮询会话建立（≤10s），同步返回完整 session，
// 而不是 pending 状态；因此保留 pollSession 特殊逻辑。
func (l *MemoryMatchBuyLogic) MemoryMatchBuy(req *types.MemoryMatchPurchaseReq) (*types.MemoryMatchPurchaseResp, error) {
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
	// 单次查询不烧请求预算；PENDING=并发在途，交由 Submit 同 gid 冲突回查兜底）
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

	// ③ 支付方式：仅 WALLET（外卡 6021）
	method := strings.ToUpper(strings.TrimSpace(req.PaymentMethod))
	if method == "" {
		method = "WALLET"
	}
	if method != "WALLET" {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchPaymentUnsupported, "支付方式暂不支持，仅支持钱包支付")
	}

	// ④ 统一购买骨架：gid → 单价 → 报价 → 预检 → 5 分支
	flow := &saga.BuyFlow{
		Ctx:              l.ctx,
		Log:              l.Logger,
		Svc:              l.svcCtx,
		RequestID:        l.requestId,
		RequireRequestID: true,
		Scene:            "MEMORY_MATCH",
		ConfigID:         req.ConfigId,
		FailMsg:          "记忆翻牌请求提交失败，请稍后重试",
		UserCouponID:     req.UserCouponId,
		LoadUnitFen: func(ctx context.Context, userID int64) (int64, error) {
			detail, err := l.svcCtx.PlayRpc.GetMemoryMatchConfig(ctx, &playClient.GetMemoryMatchConfigReq{
				ConfigId: req.ConfigId,
				UserId:   userID,
			})
			if err != nil {
				l.Errorf("MemoryMatchBuy GetMemoryMatchConfig failed, configId: %d, err: %v", req.ConfigId, err)
				return 0, errorx.FromError(err)
			}
			unitFen, err := utils.YuanToFen(detail.Config.PurchasePrice)
			if err != nil || unitFen <= 0 {
				return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "记忆翻牌价格无效")
			}
			return unitFen, nil
		},
		BuildBranches: func(gid string, userID int64, quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]saga.Branch, error) {
			c := l.svcCtx.Config
			return saga.PlayBranches(c.MoneyRpcDirect, gid, userID, req.ConfigId, quote, saga.PlayBranchSet{
				// 01 锁报价+会话插入 PENDING；补偿 LockMemoryMatchRevert 会话 EXPIRED+释放报价
				LockPath:    c.PlayRpcDirect + "/play.Play/LockMemoryMatch",
				ReleasePath: c.PlayRpcDirect + "/play.Play/LockMemoryMatchRevert",
				LockPayload: &playClient.LockMemoryMatchReq{
					RequestId:     gid,
					QuoteToken:    quote.QuoteToken,
					ConfigId:      req.ConfigId,
					UserId:        userID,
					FacePresetId:  req.FacePresetId,
					BackPresetId:  req.BackPresetId,
					TotalPriceFen: unitFen,
				},
				// 03 PENDING→AWAITING_DESTINY；补偿 SettleMemoryMatchRevert
				SettlePath:       c.PlayRpcDirect + "/play.Play/SettleMemoryMatch",
				SettleRevertPath: c.PlayRpcDirect + "/play.Play/SettleMemoryMatchRevert",
				SettlePayload: &playClient.SettleMemoryMatchReq{
					RequestId: gid,
					UserId:    userID,
					ConfigId:  req.ConfigId,
					SessionId: 0,
					Price:     utils.FenToYuan(quote.PayableFen),
				},
				// 04 发奖空操作（购买时无奖，终局结算同步发奖；永不 Aborted、空补偿）
				GrantPath:       c.PlayRpcDirect + "/play.Play/GrantMemoryMatchPrizes",
				GrantRevertPath: c.PlayRpcDirect + "/play.Play/GrantMemoryMatchPrizesRevert",
				GrantPayload: &playClient.GrantMemoryMatchPrizesReq{
					RequestId: gid,
					UserId:    userID,
					SessionId: 0,
					ConfigId:  req.ConfigId,
				},
			}), nil
		},
		// 不传 Recheck：Submit 失败由下面的会话轮询兜底（老实现同款）
	}

	gid, userID, _, err := flow.Run()
	if err != nil {
		// Submit 失败很可能是「同 gid 事务已存在」：轮询会话兜底
		fallbackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if sess, found, _ := l.pollSession(fallbackCtx, gid, userID); found {
			l.Infof("MemoryMatchBuy submit failed but session exists, gid: %s, sessionId: %d", gid, sess.SessionId)
			l.applyTaskProgress(userID, gid, sess.PaidMinor)
			return l.successResp(sess), nil
		}
		l.Errorf("dtm saga submit failed, gid: %s, err: %v", gid, err)
		return nil, err
	}
	l.Infof("dtm saga submitted, gid: %s, configId: %d, userId: %d", gid, req.ConfigId, userID)

	// ⑤ 轮询会话建立（≤10s）：同步返回 sessionId/state/appearance
	pollCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if sess, found, expired := l.pollSession(pollCtx, gid, userID); found {
		if expired {
			l.Errorf("MemoryMatchBuy saga compensated, gid: %s", gid)
			return nil, errorx.NewCodeError(errorx.CodeDTMError, "购买未完成（事务已回滚），请更换幂等键重试")
		}
		l.applyTaskProgress(userID, gid, sess.PaidMinor)
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
