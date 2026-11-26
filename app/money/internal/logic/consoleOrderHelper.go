package logic

import (
	"time"

	"gacha_srv/app/money/internal/repo"
	"gacha_srv/app/money/money"
)

func consolePageSize(n int32) int {
	if n <= 0 {
		return 15
	}
	if n > 100 {
		return 100
	}
	return int(n)
}

func consoleTimeMillis(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixMilli()
}

func rechargeOrderToProto(row *repo.RechargeOrder) *money.RechargeOrder {
	return &money.RechargeOrder{
		DbId:                  row.Id,
		OrderNo:               row.OrderNo,
		UserId:                row.UserId,
		UserNickname:          row.UserNickname,
		AmountFen:             row.AmountFen,
		CreditedFen:           row.CreditedFen,
		DiscountFen:           row.DiscountFen,
		Platform:              row.Platform,
		Status:                money.RechargeOrderStatus(row.Status),
		RechargeType:          row.RechargeType,
		CreatedAtUnixMillis:   row.CreatedAt.UnixMilli(),
		CompletedAtUnixMillis: consoleTimeMillis(row.CompletedAt),
		RefundedAtUnixMillis:  consoleTimeMillis(row.RefundedAt),
		RefundAmountFen:       row.RefundAmountFen,
		RefundOrderNo:         row.RefundOrderNo,
		PayPlatformOrderId:    row.PayPlatformOrderId,
		PayPlatformOpenid:     row.PayPlatformOpenid,
		ErrorMessage:          row.ErrorMessage,
	}
}

func refundOrderToProto(row *repo.RefundOrder) *money.RefundOrder {
	return &money.RefundOrder{
		DbId:                  row.Id,
		OrderNo:               row.OrderNo,
		UserId:                row.UserId,
		UserNickname:          row.UserNickname,
		RealName:              row.RealName,
		AlipayAccount:         row.AlipayAccount,
		AmountFen:             row.AmountFen,
		Status:                money.RefundOrderStatus(row.Status),
		CreatedAtUnixMillis:   row.CreatedAt.UnixMilli(),
		CompletedAtUnixMillis: consoleTimeMillis(row.CompletedAt),
		ThirdPartyOrderId:     row.ThirdPartyOrderId,
		RefundMerchantId:      row.RefundMerchantId,
		Auditor:               row.Auditor,
		FinanceAuditor:        row.FinanceAuditor,
		AuditRemark:           row.AuditRemark,
		ErrorMessage:          row.ErrorMessage,
	}
}

func refundAutoConfigToProto(row *repo.RefundAutoConfig) *money.RefundAutoConfig {
	return &money.RefundAutoConfig{
		RemainingAmountFen: row.RemainingAmountFen,
		AuditLimitFen:      row.AuditLimitFen,
		DelaySeconds:       row.DelaySeconds,
		AlternateCount:     row.AlternateCount,
		AlternateAmountFen: row.AlternateAmountFen,
	}
}
