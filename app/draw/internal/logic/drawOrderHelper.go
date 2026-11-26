package logic

import (
	"fmt"
	"strings"
	"time"

	"gacha_srv/pkg/utils/ordercode"

	"gorm.io/gorm"
)

// DrawOrderAllocation 抽盒订单发号结果（requestId 幂等）。
type DrawOrderAllocation struct {
	RequestID string
	OrderID   int64
	OrderSeq  int64
	OrderNo   string
}

// allocateDrawOrder 分配内部订单号：YYYYMMDD*10^9 + 序号（从 333000000 起全局递增）。
// 同一 requestId 重复调用返回同一结果；并发冲突时回读已分配行。
func allocateDrawOrder(tx *gorm.DB, requestID string) (*DrawOrderAllocation, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 64 {
		return nil, fmt.Errorf("requestId 必填且不超过 64 字符")
	}

	var existing DrawOrderAllocation
	if err := tx.Raw(
		"SELECT request_id, order_id, order_seq, order_no FROM draw_order_allocations WHERE request_id = ?",
		requestID,
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

	alloc := &DrawOrderAllocation{
		RequestID: requestID,
		OrderID:   orderID,
		OrderSeq:  orderSeq,
		OrderNo:   orderNo,
	}
	if err = tx.Exec(
		"INSERT INTO draw_order_allocations (request_id, order_id, order_seq, order_no, created_at) VALUES (?, ?, ?, ?, NOW(3))",
		alloc.RequestID, alloc.OrderID, alloc.OrderSeq, alloc.OrderNo,
	).Error; err != nil {
		// 并发同 requestId：回读已提交的分配结果（锁定读，避免快照看不到新行）。
		var prior DrawOrderAllocation
		if qerr := tx.Raw(
			"SELECT request_id, order_id, order_seq, order_no FROM draw_order_allocations WHERE request_id = ? FOR UPDATE",
			requestID,
		).Scan(&prior).Error; qerr == nil && prior.OrderID > 0 {
			return &prior, nil
		}
		return nil, err
	}
	return alloc, nil
}
