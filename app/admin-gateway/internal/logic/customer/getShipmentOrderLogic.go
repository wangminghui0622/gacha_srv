// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/draw/draw"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetShipmentOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetShipmentOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetShipmentOrderLogic {
	return &GetShipmentOrderLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetShipmentOrderLogic) GetShipmentOrder(req *draw.GetShipmentOrderReq) (*draw.ShipmentOrder, error) {
	return l.svcCtx.DrawRpc.GetShipmentOrder(l.ctx, req)
}
