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

// BuyFlow 6 个玩法购买流程的统一骨架（draw 抽盒走独立 SubmitDrawSaga，不在此列）。
//
// 固定顺序：
//
//	userId（JWT）→ gid（Idempotency-Key/uuid）→ 幂等快路径 → 服务端单价
//	→ 报价 QuoteCheckout → 同步预检 Precheck → 5 分支 → Submit(+回查)
//
// 玩法差异全部收敛到 4 个 hook：
//   - LoadUnitFen：取服务端单价（不信客户端）
//   - AmountFen：总价（默认单价；count 类玩法传 unit*count）
//   - Idempotent：同 gid 已有记录 → 直接 success（可选）
//   - BuildBranches：5 个分支的地址 + 类型化 payload
//   - Recheck：Submit 失败时按 gid 回查记录（可选）
type BuyFlow struct {
	Ctx       context.Context
	Log       logx.Logger
	Svc       *svc.ServiceContext
	RequestID string // HTTP Idempotency-Key（可选）
	Scene     string // 资格/风控场景：GASHAPON / FULL_CAR / ...
	ConfigID  int64
	FailMsg   string

	// UserCouponID / UseLoyaltyOffset 仅 memorymatch 等支持券的玩法使用；其它留零值。
	UserCouponID     int64
	UseLoyaltyOffset bool

	LoadUnitFen func(ctx context.Context, userID int64) (int64, error)
	// Prepare 可选：读取/解析玩法特有状态（如挖宝格子），在单价之后、报价之前执行。
	Prepare  func(ctx context.Context, userID int64) error
	AmountFen func(unitFen int64) int64 // nil = unitFen
	Idempotent  func(ctx context.Context, gid string, userID int64) (bool, error)
	BuildBranches func(gid string, userID int64, quote *marketingClient.QuoteCheckoutResp,
		unitFen, grossFen int64) ([]Branch, error)
	Recheck func(ctx context.Context, gid string, userID int64) error
}

// Run 执行购买流程，返回 gid / 是否幂等命中 / 错误。
func (f *BuyFlow) Run() (gid string, userID int64, alreadyDone bool, err error) {
	userID, ok := middleware.UserIdFromCtx(f.Ctx)
	if !ok || userID <= 0 {
		return "", 0, false, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}

	// ① gid = Idempotency-Key（重试复用）或服务端 uuid
	gid = uuid.New().String()
	if f.RequestID != "" {
		if len(f.RequestID) > 64 {
			return "", 0, false, errorx.NewCodeError(errorx.CodeParamError, "幂等键过长（最长 64）")
		}
		gid = f.RequestID
		// 幂等重放快路径：同 key 已有记录 → 直接 success（先于业务解析）
		// 查询失败不阻断（与各玩法原实现一致：只有明确命中才短路）。
		if f.Idempotent != nil {
			if done, qe := f.Idempotent(f.Ctx, gid, userID); qe == nil && done {
				f.Log.Infof("retry with same idempotency key, returning existing record, gid: %s", gid)
				return gid, userID, true, nil
			}
		}
	}

	// ② 服务端单价（不信客户端）
	unitFen, err := f.LoadUnitFen(f.Ctx, userID)
	if err != nil {
		return "", 0, false, err
	}
	// ②.5 玩法特有状态（挖宝格子等）：只读解析，供 AmountFen/BuildBranches 使用
	if f.Prepare != nil {
		if err := f.Prepare(f.Ctx, userID); err != nil {
			return "", 0, false, err
		}
	}
	grossFen := unitFen
	if f.AmountFen != nil {
		grossFen = f.AmountFen(unitFen)
	}

	// ③ 报价预计算：marketing 服务端算价（幂等 uk request_id=gid）
	quote, err := f.Svc.MarketingRpc.QuoteCheckout(f.Ctx, &marketingClient.QuoteCheckoutReq{
		RequestId:        gid,
		UserId:           userID,
		BoxId:            f.ConfigID,
		UserCouponId:     f.UserCouponID,
		UseLoyaltyOffset: f.UseLoyaltyOffset,
		GrossFen:         grossFen,
	})
	if err != nil {
		f.Log.Errorf("%s QuoteCheckout failed, gid: %s, err: %v", f.Scene, gid, err)
		return "", 0, false, errorx.FromError(err)
	}

	// ④ 同步预检：资格 + 风控（拒绝立即返回，不入分布式事务）
	if err := Precheck(f.Ctx, f.Log, f.Svc.BaseRpc, gid, userID, f.ConfigID, grossFen, f.Scene); err != nil {
		return "", 0, false, err
	}

	// ⑤ 组装 5 分支 + 提交
	branches, err := f.BuildBranches(gid, userID, quote, unitFen, grossFen)
	if err != nil {
		return "", 0, false, err
	}
	b := New(f.Ctx, f.Log, f.Svc.Config.DtmServer.Target, gid)
	for _, br := range branches {
		b.Add(br.Action, br.Revert, br.Payload)
	}
	var recheck Recheck
	if f.RequestID != "" && f.Recheck != nil {
		recheck = func(ctx context.Context) error { return f.Recheck(ctx, gid, userID) }
	}
	alreadyDone, err = b.Submit(f.FailMsg, recheck)
	if err != nil {
		return "", 0, false, err
	}
	return gid, userID, alreadyDone, nil
}
