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

type GetShipmentOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetShipmentOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetShipmentOrderLogic {
	return &GetShipmentOrderLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetShipmentOrderLogic) GetShipmentOrder(in *draw.GetShipmentOrderReq) (*draw.ShipmentOrder, error) {
	if in.GetId() <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeShipmentOrderInvalid, "发货订单ID无效")
	}
	row, err := repo.GetShipmentOrder(l.svcCtx.DB, in.GetId())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeShipmentOrderNotFound, "发货订单不存在")
		}
		return nil, errorx.FromMySQLError(err)
	}
	items, err := repo.ListShipmentOrderItemsByOrder(l.svcCtx.DB, row.Id)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return shipmentOrderToProto(row, items, time.Now()), nil
}
