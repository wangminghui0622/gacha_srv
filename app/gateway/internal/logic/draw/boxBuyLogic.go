package draw

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/task/task"
	"gacha_srv/app/task/taskClient"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type BoxBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入）：
	// 重试时复用同一个 gid，DTM 屏障在 gid 维度防重
	requestId string
}

func NewBoxBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BoxBuyLogic {
	return &BoxBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *BoxBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// BoxBuy 购买开奖：网关发起 DTM SAGA 事务（参考 dtm_shop createOrderLogic 编排模式）。
//
// M10 起（M11c 抽成共享构建器 SubmitDrawSaga，BoxBuy 与活动宝箱开箱共用）：
//   同步预检：base.EvaluatePolicy 资格 + base.RiskEvaluate 风控（闸门，拒绝立即返回 4001/4002）
//   5 分支 SAGA：
//   [01 marketing.LockQuote 锁券 → 02 money.AuthorizeWalletPayment 建单+冻结 →
//    03 draw.SettleDraw 开奖 → 04 money.GrantAsset 发资产 →
//    05 money.CaptureWalletPayment 捕获核销]
//   补偿：[ReleaseQuote → CancelPayment → SettleDrawRevert → GrantAssetRevert → 空]
//
// 安全不变式：
//  1. userId 取 JWT 上下文，不接受客户端传入；
//  2. 金额取 DrawRpc.BoxDetail 的 box.price（服务端定价），不接受客户端传入；
//     券/积分抵扣由 marketing.QuoteCheckout（幂等 uk request_id=gid）服务端试算，
//     客户端只传选择项（userCouponId/useLoyaltyOffset）；02/03 全用报价实付值；
//     风控事实 amountFen 取箱价原价换算（分）。
//
// Submit 立即返回 pending，前端轮询 /draw/records?requestId= 直到出记录。
func (l *BoxBuyLogic) BoxBuy(req *types.BuyBoxReq) (resp *types.BuyBoxResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}

	// ① 金额/宝箱信息取服务端（Etcd 调 draw 服务），不信客户端
	detail, err := l.svcCtx.DrawRpc.BoxDetail(l.ctx, &drawClient.BoxDetailReq{BoxId: req.BoxId})
	if err != nil {
		l.Errorf("BoxBuy BoxDetail failed, boxId: %d, err: %v", req.BoxId, err)
		return nil, errorx.FromError(err)
	}
	// 售罄预检（缓存视图，仅快速失败 UX；SettleDraw 分支是最终裁决）
	if detail.SoldOut {
		return nil, errorx.NewCodeError(errorx.CodeBoxSoldOut, "宝箱已售罄")
	}
	// 风控事实：金额（分），与 04 冻结同源
	amountFen, err := utils.YuanToFen(detail.Price)
	if err != nil || amountFen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeAmountInvalid, "宝箱价格无效")
	}

	// ② gid = Idempotency-Key 头（重试场景复用）或服务端 uuid
	gid := uuid.New().String()
	if l.requestId != "" {
		if len(l.requestId) > 64 {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "幂等键过长（最长 64）")
		}
		gid = l.requestId
	}

	// ③ 报价预计算：marketing 服务端算价（幂等 uk request_id=gid，重试复用同报价）。
	//    券与积分互斥、门槛、积分上限都在服务端裁决，客户端只传选择项。
	quote, err := l.svcCtx.MarketingRpc.QuoteCheckout(l.ctx, &marketingClient.QuoteCheckoutReq{
		RequestId:        gid,
		UserId:           userId,
		BoxId:            req.BoxId,
		UserCouponId:     req.UserCouponId,
		UseLoyaltyOffset: req.UseLoyaltyOffset,
		GrossFen:         amountFen,
	})
	if err != nil {
		l.Errorf("BoxBuy QuoteCheckout failed, gid: %s, err: %v", gid, err)
		return nil, errorx.FromError(err)
	}

	// ④ 组装并提交 7 分支 SAGA（共享构建器：BoxBuy / CampaignBoxOpen 同款编排）
	alreadyDone, err := SubmitDrawSaga(l.ctx, l.svcCtx, l.Logger, gid, l.requestId, userId, req.BoxId, amountFen, quote)
	if err != nil {
		return nil, err
	}
	// ⑤ 购买挂钩（D9）：实付金额记入 DRAW_SPEND_MINOR 任务进度；
	//    task_event 按事件号幂等去重，重放安全；失败仅记日志不阻断购买
	l.applyTaskProgress(userId, gid, quote.PayableFen)
	if alreadyDone {
		return &types.BuyBoxResp{RequestId: gid, Status: "success"}, nil
	}
	return &types.BuyBoxResp{RequestId: gid, Status: "pending"}, nil
}

// applyTaskProgress 购买挂钩（D9）：事件号="pay:"+订单号（gid），metric=DRAW_SPEND_MINOR，
// amount=报价实付（分）。INSERT IGNORE 事件幂等；失败仅记日志（老项目事件消费失败同样重试不阻断）。
func (l *BoxBuyLogic) applyTaskProgress(userId int64, gid string, payableFen int64) {
	if payableFen <= 0 {
		return
	}
	if _, err := l.svcCtx.TaskRpc.ApplyTaskProgress(l.ctx, &taskClient.ApplyTaskProgressReq{
		UserId:  userId,
		EventId: "pay:" + gid,
		Metric:  task.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR,
		Amount:  payableFen,
	}); err != nil {
		l.Errorf("applyTaskProgress failed (non-blocking), gid: %s, userId: %d, err: %v", gid, userId, err)
	}
}
