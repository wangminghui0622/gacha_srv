package combochain

import (
	"context"

	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComboChainBuyLogic struct {
	logx.Logger
	ctx       context.Context
	svcCtx    *svc.ServiceContext
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

// ComboChainBuy 连击链购买：5 分支 SAGA，玩法差异见 purchaseMode。
func (l *ComboChainBuyLogic) ComboChainBuy(req *types.ComboChainBuyReq) (*types.ComboChainBuyResp, error) {
	gid, userID, alreadyDone, err := saga.RunBuy(l.ctx, l.Logger, l.svcCtx, l.requestId, req,
		purchaseMode{log: l.Logger})
	if err != nil {
		return nil, err
	}
	status := "pending"
	if alreadyDone {
		status = "success"
	}
	l.Infof("combo chain buy submitted, gid: %s, configId: %d, userId: %d, status: %s",
		gid, req.ConfigId, userID, status)
	return &types.ComboChainBuyResp{RequestId: gid, Status: status}, nil
}
