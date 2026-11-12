package saga

import (
	"context"
	"errors"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

// ErrRecordNotFound 幂等回查哨兵：Submit 失败时用来区分
// 「同 gid 事务已存在（重试成功）」与「真的没有记录（真故障）」。
var ErrRecordNotFound = errors.New("record not found")

// PurchaseMode 一个玩法的购买策略接口（泛型 Req = 该玩法的 HTTP 请求类型）。
//
// 网关只负责通用编排：
//
//	userId → gid → 幂等 → 单价 → Prepare → 报价 → 预检 → 分支 → Submit
//
// 玩法只实现这些方法。新增玩法 = 加一个 PurchaseMode 实现，不改 RunBuy。
type PurchaseMode[Req any] interface {
	// Name 玩法标识（日志/指标用）。
	Name() string
	// Scene 资格/风控场景：GASHAPON / FULL_CAR / ...
	Scene() string
	// ConfigID 该玩法的配置/箱子 ID（用于报价、预检）。
	ConfigID(req Req) int64
	// FailMsg Submit 失败时的用户可见文案。
	FailMsg() string
	// RequireRequestID 是否强制 Idempotency-Key（memorymatch=true）。
	RequireRequestID() bool
	// UserCouponID 报价使用的用户券 ID（不支持券的玩法返回 0）。
	UserCouponID(req Req) int64
	// LoadUnitFen 取服务端单价（分），不信客户端。
	LoadUnitFen(ctx context.Context, svcCtx *svc.ServiceContext, req Req, userID int64) (int64, error)
	// AmountFen 总价（分）；默认返回 unitFen，count 类玩法返回 unitFen*count。
	AmountFen(unitFen int64, req Req) int64
	// Prepare 可选：解析玩法特有状态（如挖宝格子），在报价前执行。
	Prepare(ctx context.Context, svcCtx *svc.ServiceContext, req Req, userID int64) error
	// Idempotent 同 gid 已有记录 → true（幂等快路径）。
	Idempotent(ctx context.Context, svcCtx *svc.ServiceContext, req Req, gid string, userID int64) (bool, error)
	// BuildBranches 组装 lock/settle/grant 三个业务分支（资金分支由 PlayBranches 统一插入）。
	BuildBranches(ctx context.Context, svcCtx *svc.ServiceContext, req Req, gid string, userID int64,
		quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]Branch, error)
	// Recheck Submit 失败时按 gid 回查记录；无记录返回 ErrRecordNotFound。
	Recheck(ctx context.Context, svcCtx *svc.ServiceContext, req Req, gid string, userID int64) error
}

// RunBuy 通用购买编排。玩法差异全部收敛到 PurchaseMode 实现里。
func RunBuy[Req any](ctx context.Context, log logx.Logger, svcCtx *svc.ServiceContext,
	requestID string, req Req, mode PurchaseMode[Req]) (gid string, userID int64, alreadyDone bool, err error) {
	userID, ok := middleware.UserIdFromCtx(ctx)
	if !ok || userID <= 0 {
		return "", 0, false, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	if mode.RequireRequestID() && requestID == "" {
		return "", 0, false, errorx.NewCodeError(errorx.CodeParamError, "Idempotency-Key 头必填（最长 64）")
	}

	// ① gid = Idempotency-Key（重试复用）或服务端 uuid
	gid = uuid.New().String()
	if requestID != "" {
		if len(requestID) > 64 {
			return "", 0, false, errorx.NewCodeError(errorx.CodeParamError, "幂等键过长（最长 64）")
		}
		gid = requestID
		// 幂等重放快路径：同 key 已有记录 → 直接 success（先于业务解析）
		if done, qe := mode.Idempotent(ctx, svcCtx, req, gid, userID); qe == nil && done {
			log.Infof("%s retry with same idempotency key, returning existing record, gid: %s", mode.Name(), gid)
			return gid, userID, true, nil
		}
	}

	// ② 服务端单价（不信客户端）
	unitFen, err := mode.LoadUnitFen(ctx, svcCtx, req, userID)
	if err != nil {
		return "", 0, false, err
	}
	// ②.5 玩法特有状态（挖宝格子等）
	if err := mode.Prepare(ctx, svcCtx, req, userID); err != nil {
		return "", 0, false, err
	}
	grossFen := mode.AmountFen(unitFen, req)

	// ③ 报价预计算：marketing 服务端算价（幂等 uk request_id=gid）
	quote, err := svcCtx.MarketingRpc.QuoteCheckout(ctx, &marketingClient.QuoteCheckoutReq{
		RequestId:    gid,
		UserId:       userID,
		BoxId:        mode.ConfigID(req),
		UserCouponId: mode.UserCouponID(req),
		GrossFen:     grossFen,
	})
	if err != nil {
		log.Errorf("%s QuoteCheckout failed, gid: %s, err: %v", mode.Name(), gid, err)
		return "", 0, false, errorx.FromError(err)
	}

	// ④ 同步预检：资格 + 风控（拒绝立即返回，不入分布式事务）
	if err := Precheck(ctx, log, svcCtx.BaseRpc, gid, userID, mode.ConfigID(req), grossFen, mode.Scene()); err != nil {
		return "", 0, false, err
	}

	// ⑤ 组装 5 分支 + 提交
	branches, err := mode.BuildBranches(ctx, svcCtx, req, gid, userID, quote, unitFen, grossFen)
	if err != nil {
		return "", 0, false, err
	}
	b := New(ctx, log, svcCtx.Config.DtmServer.Target, gid)
	for _, br := range branches {
		b.AddBranch(br)
	}
	var recheck Recheck
	if requestID != "" {
		recheck = func(rcCtx context.Context) error {
			return mode.Recheck(rcCtx, svcCtx, req, gid, userID)
		}
	}
	alreadyDone, err = b.Submit(mode.FailMsg(), recheck)
	if err != nil {
		return "", 0, false, err
	}
	return gid, userID, alreadyDone, nil
}
