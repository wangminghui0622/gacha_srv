// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/draw/draw"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListShipmentOrdersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListShipmentOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListShipmentOrdersLogic {
	return &ListShipmentOrdersLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListShipmentOrdersLogic) ListShipmentOrders(req *draw.ListShipmentOrdersReq) (*draw.ListShipmentOrdersResp, error) {
	return l.svcCtx.DrawRpc.ListShipmentOrders(l.ctx, req)
}
