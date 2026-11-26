package logic

import (
	"fmt"
	"strings"
	"time"

	"gacha_srv/pkg/utils/ordercode"

	"gorm.io/gorm"
)

// DrawOrderRow 抽盒订单主表行（draw_orders）。
type DrawOrderRow struct {
	OrderID   int64
	RequestID string
	OrderNo   string
	OrderSeq  int64
	UserID    int64
	BoxID     int64
	PriceFen  int64
	Status    int32
	RecordID  int64
}

// ensureDrawOrder 按 requestId 幂等创建订单主表行。
// 订单号 = YYYYMMDD*10^9 + 序号（从 333000000 起全局递增）；
// 对外订单号 = o + Base62(序号)。
func ensureDrawOrder(tx *gorm.DB, requestID string, userID, boxID, priceFen int64) (*DrawOrderRow, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 64 {
		return nil, fmt.Errorf("requestId 必填且不超过 64 字符")
	}

	var existing DrawOrderRow
	if err := tx.Raw(
		"SELECT id AS order_id, request_id, order_no, order_seq, user_id, box_id, price_fen, status, record_id "+
			"FROM draw_orders WHERE request_id = ?", requestID,
	).Scan(&existing).Error; err != nil {
		return nil, err
	}
	if existing.OrderID > 0 {
		return &existing, nil
	}

	// AUTO_INCREMENT 表发号：并发安全，值从 333000000 开始。
	if err := tx.Exec("INSERT INTO draw_order_sequences (created_at) VALUES (NOW(3))").Error; err != nil {
		return nil, err
	}
	var orderSeq int64
	if err := tx.Raw("SELECT LAST_INSERT_ID()").Scan(&orderSeq).Error; err != nil {
		return nil, err
	}
	if orderSeq <= 0 {
		return nil, fmt.Errorf("订单序号分配失败")
	}

	// 日期前缀使用东八区，避免容器 UTC 跨天。
	now := time.Now().In(time.FixedZone("CST", 8*3600))
	dateNumber := int64(now.Year())*10000 + int64(now.Month())*100 + int64(now.Day())
	orderID := dateNumber*1_000_000_000 + orderSeq

	orderNo, err := ordercode.Encode(uint64(orderSeq))
	if err != nil {
		return nil, err
	}

	order := &DrawOrderRow{
		OrderID:   orderID,
		RequestID: requestID,
		OrderNo:   orderNo,
		OrderSeq:  orderSeq,
		UserID:    userID,
		BoxID:     boxID,
		PriceFen:  priceFen,
		Status:    1,
	}
	if err = tx.Exec(
		"INSERT INTO draw_orders (id, request_id, order_no, order_seq, user_id, box_id, price_fen, status, created_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, 1, NOW(3))",
		order.OrderID, order.RequestID, order.OrderNo, order.OrderSeq,
		order.UserID, order.BoxID, order.PriceFen,
	).Error; err != nil {
		// 并发同 requestId：锁定读回已提交订单。
		var prior DrawOrderRow
		if qerr := tx.Raw(
			"SELECT id AS order_id, request_id, order_no, order_seq, user_id, box_id, price_fen, status, record_id "+
				"FROM draw_orders WHERE request_id = ? FOR UPDATE", requestID,
		).Scan(&prior).Error; qerr == nil && prior.OrderID > 0 {
			return &prior, nil
		}
		return nil, err
	}
	return order, nil
}

// markDrawOrderCompleted 开奖成功：订单状态置完成并回写开奖记录。
func markDrawOrderCompleted(tx *gorm.DB, orderID, recordID int64) error {
	return tx.Exec(
		"UPDATE draw_orders SET status = 2, record_id = ?, updated_at = NOW(3) WHERE id = ?",
		recordID, orderID,
	).Error
}

// markDrawOrderCanceled 开奖补偿：订单状态置取消/失败（已完成订单也要撤销，整单回滚）。
func markDrawOrderCanceled(tx *gorm.DB, orderID int64) error {
	return tx.Exec(
		"UPDATE draw_orders SET status = 3, updated_at = NOW(3) WHERE id = ?",
		orderID,
	).Error
}
