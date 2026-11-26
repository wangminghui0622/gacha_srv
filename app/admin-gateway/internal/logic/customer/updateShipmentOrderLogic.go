// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/draw/draw"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateShipmentOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateShipmentOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateShipmentOrderLogic {
	return &UpdateShipmentOrderLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateShipmentOrderLogic) UpdateShipmentOrder(req *draw.UpdateShipmentOrderReq) (*draw.ShipmentOrder, error) {
	return l.svcCtx.DrawRpc.UpdateShipmentOrder(l.ctx, req)
}
