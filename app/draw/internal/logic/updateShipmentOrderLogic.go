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

type UpdateShipmentOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateShipmentOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateShipmentOrderLogic {
	return &UpdateShipmentOrderLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateShipmentOrderLogic) UpdateShipmentOrder(in *draw.UpdateShipmentOrderReq) (*draw.ShipmentOrder, error) {
	if in.GetId() <= 0 || !shipmentStatusValid(in.GetStatus()) {
		return nil, errorx.NewCodeError(errorx.CodeShipmentOrderInvalid, "发货订单ID/状态无效")
	}
	row, err := repo.GetShipmentOrder(l.svcCtx.DB, in.GetId())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeShipmentOrderNotFound, "发货订单不存在")
		}
		return nil, errorx.FromMySQLError(err)
	}

	now := time.Now()
	shippedAt := shipmentMillisTime(in.GetShippedAtUnixMillis())
	completedAt := shipmentMillisTime(in.GetCompletedAtUnixMillis())
	switch draw.ShipmentOrderStatus(in.GetStatus()) {
	case draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_PENDING_RECEIPT:
		if shippedAt == nil && row.ShippedAt == nil {
			shippedAt = &now
		}
	case draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_COMPLETED:
		if shippedAt == nil && row.ShippedAt == nil {
			shippedAt = &now
		}
		if completedAt == nil {
			completedAt = &now
		}
	}

	updates := map[string]any{
		"status":            in.GetStatus(),
		"remark":            in.GetRemark(),
		"receiver_name":     in.GetReceiverName(),
		"receiver_phone":    in.GetReceiverPhone(),
		"receiver_address":  in.GetReceiverAddress(),
		"logistics_company": in.GetLogisticsCompany(),
		"tracking_no":       in.GetTrackingNo(),
		"shipped_at":        shippedAt,
		"completed_at":      completedAt,
	}
	if in.GetStatus() == int32(draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_CLOSED) ||
		in.GetStatus() == int32(draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_USER_CLOSED) {
		updates["closed_at"] = &now
	} else {
		updates["closed_at"] = nil
	}
	if err := repo.UpdateShipmentOrder(l.svcCtx.DB, row.Id, updates); err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return NewGetShipmentOrderLogic(l.ctx, l.svcCtx).GetShipmentOrder(&draw.GetShipmentOrderReq{Id: row.Id})
}
