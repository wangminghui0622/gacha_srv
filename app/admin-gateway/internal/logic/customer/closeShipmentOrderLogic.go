// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/draw/draw"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseShipmentOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCloseShipmentOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseShipmentOrderLogic {
	return &CloseShipmentOrderLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CloseShipmentOrderLogic) CloseShipmentOrder(req *draw.CloseShipmentOrderReq) (*draw.ShipmentOrder, error) {
	return l.svcCtx.DrawRpc.CloseShipmentOrder(l.ctx, req)
}
