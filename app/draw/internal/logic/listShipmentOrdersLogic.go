package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/repo"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListShipmentOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListShipmentOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListShipmentOrdersLogic {
	return &ListShipmentOrdersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListShipmentOrdersLogic) ListShipmentOrders(in *draw.ListShipmentOrdersReq) (*draw.ListShipmentOrdersResp, error) {
	page := int(in.GetPage())
	if page <= 0 {
		page = 1
	}
	pageSize := clampPageSize(in.GetPageSize())
	keyword := strings.TrimSpace(in.GetKeyword())

	var (
		rows  []repo.ShipmentOrder
		total int64
		err   error
	)
	if userId, parseErr := strconv.ParseInt(keyword, 10, 64); keyword != "" && parseErr == nil && userId > 0 {
		rows, total, err = repo.ListShipmentOrdersByUserId(l.svcCtx.DB, keyword, userId, in.GetStatus(), (page-1)*pageSize, pageSize)
	} else {
		rows, total, err = repo.ListShipmentOrders(l.svcCtx.DB, keyword, in.GetStatus(), (page-1)*pageSize, pageSize)
	}
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}

	orderIDs := make([]int64, 0, len(rows))
	for i := range rows {
		orderIDs = append(orderIDs, rows[i].Id)
	}
	items, err := repo.ListShipmentOrderItems(l.svcCtx.DB, orderIDs)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	itemsByOrder := make(map[int64][]repo.ShipmentOrderItem, len(rows))
	for i := range items {
		itemsByOrder[items[i].OrderId] = append(itemsByOrder[items[i].OrderId], items[i])
	}

	now := time.Now()
	resp := &draw.ListShipmentOrdersResp{Total: total}
	for i := range rows {
		resp.Items = append(resp.Items, shipmentOrderToProto(&rows[i], itemsByOrder[rows[i].Id], now))
	}
	return resp, nil
}
