package logic

import (
	"time"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/repo"
)

// shipmentStatusValid 后台可设置的发货订单状态。
func shipmentStatusValid(status int32) bool {
	return status >= int32(draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_PENDING_PAYMENT) &&
		status <= int32(draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_USER_CLOSED)
}

// shipmentTimeMillis 时间 → unix 毫秒（空=0）。
func shipmentTimeMillis(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixMilli()
}

// shipmentMillisTime unix 毫秒 → 时间（<=0=清空）。
func shipmentMillisTime(ms int64) *time.Time {
	if ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms)
	return &t
}

// shipmentSubmittedDays 已提交天数：仅待发货/待收货显示，基准为提交时间（回退支付/创建时间）。
func shipmentSubmittedDays(row *repo.ShipmentOrder, now time.Time) int32 {
	if row.Status != int32(draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_PENDING_SHIPMENT) &&
		row.Status != int32(draw.ShipmentOrderStatus_SHIPMENT_ORDER_STATUS_PENDING_RECEIPT) {
		return 0
	}
	base := row.SubmittedAt
	if base == nil {
		base = row.PaidAt
	}
	if base == nil {
		base = &row.CreatedAt
	}
	days := int(now.Sub(*base).Hours() / 24)
	if days < 0 {
		return 0
	}
	return int32(days)
}

// shipmentOrderToProto 订单行 + 物品行 → proto 视图。
func shipmentOrderToProto(row *repo.ShipmentOrder, items []repo.ShipmentOrderItem, now time.Time) *draw.ShipmentOrder {
	out := &draw.ShipmentOrder{
		Id:                    row.Id,
		OrderNo:               row.OrderNo,
		UserId:                row.UserId,
		Status:                draw.ShipmentOrderStatus(row.Status),
		CreatedAtUnixMillis:   row.CreatedAt.UnixMilli(),
		PaidAtUnixMillis:      shipmentTimeMillis(row.PaidAt),
		SubmittedDays:         shipmentSubmittedDays(row, now),
		ShippedAtUnixMillis:   shipmentTimeMillis(row.ShippedAt),
		CompletedAtUnixMillis: shipmentTimeMillis(row.CompletedAt),
		Remark:                row.Remark,
		ReceiverName:          row.ReceiverName,
		ReceiverPhone:         row.ReceiverPhone,
		ReceiverAddress:       row.ReceiverAddress,
		LogisticsCompany:      row.LogisticsCompany,
		TrackingNo:            row.TrackingNo,
		ItemCount:             int32(len(items)),
	}
	for i := range items {
		item := &items[i]
		out.Items = append(out.Items, &draw.ShipmentOrderItem{
			Id:                  item.Id,
			WarehouseItemId:     item.WarehouseItemId,
			BoxId:               item.BoxId,
			ItemName:            item.ItemName,
			ImageUrl:            item.ImageUrl,
			Quantity:            item.Quantity,
			CreatedAtUnixMillis: item.CreatedAt.UnixMilli(),
		})
	}
	return out
}
