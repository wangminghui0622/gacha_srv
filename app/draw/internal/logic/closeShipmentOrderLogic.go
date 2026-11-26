package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/repo"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type CloseShipmentOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseShipmentOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseShipmentOrderLogic {
	return &CloseShipmentOrderLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CloseShipmentOrderLogic) CloseShipmentOrder(in *draw.CloseShipmentOrderReq) (*draw.ShipmentOrder, error) {
	if in.GetId() <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeShipmentOrderInvalid, "发货订单ID无效")
	}
	status := in.GetStatus()
	if status != int32(draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_CLOSED) &&
		status != int32(draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_USER_CLOSED) {
		return nil, errorx.NewCodeError(errorx.CodeShipmentOrderInvalid, "关闭状态只能为已关闭或用户关闭")
	}
	row, err := repo.GetShipmentOrder(l.svcCtx.DB, in.GetId())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeShipmentOrderNotFound, "发货订单不存在")
		}
		return nil, errorx.FromMySQLError(err)
	}
	if row.Status == int32(draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_COMPLETED) {
		return nil, errorx.NewCodeError(errorx.CodeShipmentOrderInvalid, "已完成订单不能关闭")
	}
	now := time.Now()
	if err := repo.UpdateShipmentOrder(l.svcCtx.DB, row.Id, map[string]any{
		"status":    status,
		"closed_at": &now,
	}); err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return NewGetShipmentOrderLogic(l.ctx, l.svcCtx).GetShipmentOrder(&draw.GetShipmentOrderReq{Id: row.Id})
}
