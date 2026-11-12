package dig

import (
	"context"

	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DigBuyLogic struct {
	logx.Logger
	ctx       context.Context
	svcCtx    *svc.ServiceContext
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

// DigBuy 挖宝购买：5 分支 SAGA，格子解析/单价/分支见 purchaseMode。
//
// 幂等快路径必须在格子解析之前——全开后重放时 resolveDigCells 会先报 6002。
func (l *DigBuyLogic) DigBuy(req *types.DigBuyReq) (*types.DigBuyResp, error) {
	mode := &purchaseMode{log: l.Logger}
	gid, userID, alreadyDone, err := saga.RunBuy(l.ctx, l.Logger, l.svcCtx, l.requestId, req, mode)
	if err != nil {
		return nil, err
	}
	status := "pending"
	if alreadyDone {
		status = "success"
	}
	l.Infof("dig buy submitted, gid: %s, boxId: %d, userId: %d, cells: %d, status: %s",
		gid, req.BoxId, userID, len(mode.cells), status)
	return &types.DigBuyResp{RequestId: gid, Status: status}, nil
}
