package fullcar

import (
	"context"

	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FullCarBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入）
	requestId string
}

func NewFullCarBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FullCarBuyLogic {
	return &FullCarBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *FullCarBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// FullCarBuy 满车购座：网关发起 DTM SAGA 事务（5 分支）。
//
//	[01 play.LockFullCar 锁报价+占座+余票-1 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleFullCar 抽 regular 奖+落记录（最后一票触发整车结算）
//	 → 04 play.GrantFullCarPrizes 发整车奖（永不 Aborted，空补偿）
//	 → 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseFullCar → CancelPayment → SettleFullCarRevert → 空 → 空]
func (l *FullCarBuyLogic) FullCarBuy(req *types.FullCarBuyReq) (*types.FullCarBuyResp, error) {
	gid, userID, alreadyDone, err := saga.RunBuy(l.ctx, l.Logger, l.svcCtx, l.requestId, req,
		purchaseMode{log: l.Logger})
	if err != nil {
		return nil, err
	}
	status := "pending"
	if alreadyDone {
		status = "success"
	}
	l.Infof("full car buy submitted, gid: %s, configId: %d, userId: %d, seatIndex: %d, status: %s",
		gid, req.ConfigId, userID, req.SeatIndex, status)
	return &types.FullCarBuyResp{RequestId: gid, Status: status}, nil
}
