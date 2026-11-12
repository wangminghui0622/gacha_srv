package gashapon

import (
	"context"

	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

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

// GashaponBuy 扭蛋购买：网关发起 DTM SAGA 事务。
//
//	[01 play.LockGashapon 锁报价+参与记录+余票-count
//	 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleGashapon 抽 count 个 regular 奖+特别奖判定+落记录
//	 → 04 play.GrantGashaponPrizes 发特别奖（永不 Aborted，空补偿）
//	 → 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseGashapon → CancelPayment → SettleGashaponRevert → 空 → 空]
//
// 玩法差异（单价/幂等/分支）在 purchaseMode 中实现，通用编排见 saga.RunBuy。
func (l *GashaponBuyLogic) GashaponBuy(req *types.GashaponBuyReq) (*types.GashaponBuyResp, error) {
	if req.Count <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "抽数非法")
	}

	gid, userID, alreadyDone, err := saga.RunBuy(l.ctx, l.Logger, l.svcCtx, l.requestId, req,
		purchaseMode{log: l.Logger})
	if err != nil {
		return nil, err
	}
	status := "pending"
	if alreadyDone {
		status = "success"
	}
	l.Infof("gashapon buy submitted, gid: %s, configId: %d, userId: %d, count: %d, status: %s",
		gid, req.ConfigId, userID, req.Count, status)
	return &types.GashaponBuyResp{RequestId: gid, Status: status}, nil
}
