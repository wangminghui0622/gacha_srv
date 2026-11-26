package report

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// RevenueRow 营收报表每日明细（金额单位：分，比率单位：万分比）。
type RevenueRow struct {
	Date            string `json:"date"`
	Registrations   int64  `json:"registrations"`
	ActiveUsers     int64  `json:"activeUsers"`
	NewActiveUsers  int64  `json:"newActiveUsers"`
	OldActiveUsers  int64  `json:"oldActiveUsers"`
	ActiveUsers7d   int64  `gorm:"column:active_users_7d" json:"activeUsers7d"`
	ActiveUsers15d  int64  `gorm:"column:active_users_15d" json:"activeUsers15d"`
	Churn1dBps      int64  `gorm:"column:churn_1d_bps" json:"churn1dBps"`
	Churn3dBps      int64  `gorm:"column:churn_3d_bps" json:"churn3dBps"`
	PayAmountFen    int64  `json:"payAmountFen"`
	NewPayAmountFen int64  `json:"newPayAmountFen"`
	OldPayAmountFen int64  `json:"oldPayAmountFen"`
	RefundAmountFen int64  `json:"refundAmountFen"`
	RefundUsers     int64  `json:"refundUsers"`
	PayingUsers     int64  `json:"payingUsers"`
	NewPayingUsers  int64  `json:"newPayingUsers"`
	OldPayingUsers  int64  `json:"oldPayingUsers"`
	ArpuFen         int64  `json:"arpuFen"`
	ArppuFen        int64  `json:"arppuFen"`
	PenetrationBps  int64  `json:"penetrationBps"`
	NetRevenueFen   int64  `json:"netRevenueFen"`

	WechatPayFen           int64 `gorm:"column:wechat_pay_fen" json:"wechatPayFen"`
	AlipayPayFen           int64 `gorm:"column:alipay_pay_fen" json:"alipayPayFen"`
	HuifuPayFen            int64 `gorm:"column:huifu_pay_fen" json:"huifuPayFen"`
	WechatGiftFen          int64 `gorm:"column:wechat_gift_fen" json:"wechatGiftFen"`
	AlipayGiftFen          int64 `gorm:"column:alipay_gift_fen" json:"alipayGiftFen"`
	HuifuGiftFen           int64 `gorm:"column:huifu_gift_fen" json:"huifuGiftFen"`
	WechatRefundFen        int64 `gorm:"column:wechat_refund_fen" json:"wechatRefundFen"`
	AlipayRefundFen        int64 `gorm:"column:alipay_refund_fen" json:"alipayRefundFen"`
	HuifuRefundFen         int64 `gorm:"column:huifu_refund_fen" json:"huifuRefundFen"`
	WechatGiftRefundFen    int64 `gorm:"column:wechat_gift_refund_fen" json:"wechatGiftRefundFen"`
	AlipayGiftRefundFen    int64 `gorm:"column:alipay_gift_refund_fen" json:"alipayGiftRefundFen"`
	HuifuGiftRefundFen     int64 `gorm:"column:huifu_gift_refund_fen" json:"huifuGiftRefundFen"`
	TaxFen                 int64 `gorm:"column:tax_fen" json:"taxFen"`
	NewTaxFen              int64 `gorm:"column:new_tax_fen" json:"newTaxFen"`
	OldTaxFen              int64 `gorm:"column:old_tax_fen" json:"oldTaxFen"`
	WalletBalanceFen       int64 `gorm:"column:wallet_balance_fen" json:"walletBalanceFen"`
	BonusBalanceFen        int64 `gorm:"column:bonus_balance_fen" json:"bonusBalanceFen"`
	RedeemPointBalance     int64 `gorm:"column:redeem_point_balance" json:"redeemPointBalance"`
	LoyaltyPointBalance    int64 `gorm:"column:loyalty_point_balance" json:"loyaltyPointBalance"`
	PlatformCurrencyPayFen int64 `gorm:"column:platform_currency_pay_fen" json:"platformCurrencyPayFen"`

	ArpuNewFen        int64 `gorm:"-" json:"arpuNewFen"`
	ArpuOldFen        int64 `gorm:"-" json:"arpuOldFen"`
	PenetrationNewBps int64 `gorm:"-" json:"penetrationNewBps"`
	PenetrationOldBps int64 `gorm:"-" json:"penetrationOldBps"`
	RefundRateBps     int64 `gorm:"-" json:"refundRateBps"`
	RevenueRateBps    int64 `gorm:"-" json:"revenueRateBps"`
	RevenueArpuFen    int64 `gorm:"-" json:"revenueArpuFen"`
}

// RevenueResult 营收报表。
type RevenueResult struct {
	Rows []RevenueRow `json:"rows"`
}

const revenuePayEventsSQL = `
SELECT user_id, DATE(DATE_ADD(COALESCE(completed_at, created_at), INTERVAL 8 HOUR)) AS d, amount_fen
FROM gacha_money.recharge_orders
WHERE status = 4 AND COALESCE(completed_at, created_at) >= ? AND COALESCE(completed_at, created_at) < ?`

const firstPayAllSQL = `
SELECT user_id, DATE(DATE_ADD(MIN(COALESCE(completed_at, created_at)), INTERVAL 8 HOUR)) AS d
FROM gacha_money.recharge_orders
WHERE status = 4
GROUP BY user_id`

const revenueRefundEventsSQL = `
SELECT user_id, DATE(DATE_ADD(refunded_at, INTERVAL 8 HOUR)) AS d, refund_amount_fen AS amount_fen
FROM gacha_money.recharge_orders
WHERE refund_amount_fen > 0 AND refunded_at >= ? AND refunded_at < ?
UNION ALL
SELECT user_id, DATE(DATE_ADD(completed_at, INTERVAL 8 HOUR)) AS d, amount_fen
FROM gacha_money.refund_orders
WHERE status = 3 AND completed_at >= ? AND completed_at < ?`

const revenuePlatformPaySQL = `
SELECT DATE_FORMAT(DATE_ADD(COALESCE(completed_at, created_at), INTERVAL 8 HOUR), '%Y-%m-%d') AS d,
       platform,
       COALESCE(SUM(amount_fen), 0) AS pay_fen,
       COALESCE(SUM(GREATEST(credited_fen - amount_fen, 0)), 0) AS gift_fen
FROM gacha_money.recharge_orders
WHERE status = 4 AND COALESCE(completed_at, created_at) >= ? AND COALESCE(completed_at, created_at) < ?
GROUP BY d, platform`

const revenuePlatformRefundSQL = `
SELECT DATE_FORMAT(DATE_ADD(refunded_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS d,
       platform,
       COALESCE(SUM(refund_amount_fen), 0) AS refund_fen
FROM gacha_money.recharge_orders
WHERE refund_amount_fen > 0 AND refunded_at >= ? AND refunded_at < ?
GROUP BY d, platform`

const revenueTaxSQL = `
SELECT DATE_FORMAT(DATE_ADD(r.created_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS d,
       COALESCE(SUM(r.price), 0) AS draw_fen,
       COALESCE(SUM(CASE WHEN r.is_prize = 1 THEN COALESCE(e.display_value_fen, 0) ELSE 0 END), 0) AS prize_fen
FROM gacha_draw.draw_records r
LEFT JOIN gacha_draw.draw_pool_entries e ON e.id = r.entry_id
WHERE r.status = 1 AND r.created_at >= ? AND r.created_at < ?
GROUP BY d`

const revenueWalletSQL = `
SELECT wallet_kind, COALESCE(SUM(available + held), 0) AS balance
FROM gacha_money.wallet_accounts
GROUP BY wallet_kind`

type revenueUserDay struct {
	UserID int64  `gorm:"column:user_id"`
	D      string `gorm:"column:d"`
}

type revenuePayDay struct {
	UserID    int64  `gorm:"column:user_id"`
	D         string `gorm:"column:d"`
	AmountFen int64  `gorm:"column:amount_fen"`
}

type payAgg struct {
	Users       map[int64]struct{}
	UserAmounts map[int64]int64
	Amount      int64
}

type refundAgg struct {
	Users  map[int64]struct{}
	Amount int64
}

type platformAgg struct {
	WechatPay    int64
	AlipayPay    int64
	HuifuPay     int64
	WechatGift   int64
	AlipayGift   int64
	HuifuGift    int64
	WechatRefund int64
	AlipayRefund int64
	HuifuRefund  int64
}

type revenuePlatformRow struct {
	D        string `gorm:"column:d"`
	Platform string `gorm:"column:platform"`
	PayFen   int64  `gorm:"column:pay_fen"`
	GiftFen  int64  `gorm:"column:gift_fen"`
}

type revenuePlatformRefundRow struct {
	D         string `gorm:"column:d"`
	Platform  string `gorm:"column:platform"`
	RefundFen int64  `gorm:"column:refund_fen"`
}

type revenueTaxRow struct {
	D        string `gorm:"column:d"`
	DrawFen  int64  `gorm:"column:draw_fen"`
	PrizeFen int64  `gorm:"column:prize_fen"`
}

type revenueWalletRow struct {
	WalletKind int64 `gorm:"column:wallet_kind"`
	Balance    int64 `gorm:"column:balance"`
}

// QueryRevenue 从 gacha_report 读模型查询 [start, end] 的营收报表。
func QueryRevenue(reportDB *gorm.DB, start, end time.Time) (*RevenueResult, error) {
	startLocal := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, cst)
	endLocal := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, cst)
	if endLocal.Before(startLocal) {
		return nil, fmt.Errorf("end date before start date")
	}

	var rows []RevenueRow
	if err := reportDB.Raw(`
SELECT DATE_FORMAT(report_date, '%Y-%m-%d') AS date,
       registrations, active_users, new_active_users, old_active_users,
       active_users_7d, active_users_15d, churn_1d_bps, churn_3d_bps,
       pay_amount_fen, new_pay_amount_fen, old_pay_amount_fen,
       refund_amount_fen, refund_users, paying_users,
       new_paying_users, old_paying_users, net_revenue_fen,
       wechat_pay_fen, alipay_pay_fen, huifu_pay_fen,
       wechat_gift_fen, alipay_gift_fen, huifu_gift_fen,
       wechat_refund_fen, alipay_refund_fen, huifu_refund_fen,
       wechat_gift_refund_fen, alipay_gift_refund_fen, huifu_gift_refund_fen,
       tax_fen, new_tax_fen, old_tax_fen,
       wallet_balance_fen, bonus_balance_fen, redeem_point_balance, loyalty_point_balance,
       platform_currency_pay_fen
FROM gacha_report.revenue_daily
WHERE report_date >= ? AND report_date <= ?
ORDER BY report_date DESC`,
		startLocal.Format("2006-01-02"), endLocal.Format("2006-01-02"),
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query revenue rows: %w", err)
	}
	for i := range rows {
		row := &rows[i]
		if row.ActiveUsers > 0 {
			row.ArpuFen = row.PayAmountFen / row.ActiveUsers
			row.PenetrationBps = row.PayingUsers * 10000 / row.ActiveUsers
			row.RevenueArpuFen = row.NetRevenueFen / row.ActiveUsers
		}
		if row.PayingUsers > 0 {
			row.ArppuFen = row.PayAmountFen / row.PayingUsers
		}
		if row.NewActiveUsers > 0 {
			row.ArpuNewFen = row.NewPayAmountFen / row.NewActiveUsers
			row.PenetrationNewBps = row.NewPayingUsers * 10000 / row.NewActiveUsers
		}
		if row.OldActiveUsers > 0 {
			row.ArpuOldFen = row.OldPayAmountFen / row.OldActiveUsers
			row.PenetrationOldBps = row.OldPayingUsers * 10000 / row.OldActiveUsers
		}
		if row.PayAmountFen > 0 {
			row.RefundRateBps = row.RefundAmountFen * 10000 / row.PayAmountFen
			row.RevenueRateBps = row.NetRevenueFen * 10000 / row.PayAmountFen
		}
	}
	return &RevenueResult{Rows: rows}, nil
}

// AggregateRevenue 从业务库聚合 [start, end] 的营收日报，写入 gacha_report。
func AggregateRevenue(sourceDB, reportDB *gorm.DB, start, end time.Time) error {
	startLocal := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, cst)
	endLocal := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, cst)
	today := time.Now().In(cst)
	todayLocal := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, cst)
	if endLocal.Before(startLocal) {
		return fmt.Errorf("revenue end date before start date")
	}
	if endLocal.After(todayLocal) {
		endLocal = todayLocal
	}
	if endLocal.Before(startLocal) {
		return nil
	}

	startUTC := startLocal.UTC()
	endUTC := endLocal.AddDate(0, 0, 1).UTC()
	activeStartUTC := startLocal.AddDate(0, 0, -14).UTC()
	activeEndUTC := todayLocal.AddDate(0, 0, 1).UTC()

	var regRows []revenueUserDay
	if err := sourceDB.Raw(registrationEventsSQL, startUTC, endUTC).Scan(&regRows).Error; err != nil {
		return fmt.Errorf("revenue registrations: %w", err)
	}
	var activeRows []revenueUserDay
	if err := sourceDB.Raw(activeEventsSQL,
		activeStartUTC, activeEndUTC,
		activeStartUTC, activeEndUTC,
		activeStartUTC, activeEndUTC,
		activeStartUTC, activeEndUTC,
	).Scan(&activeRows).Error; err != nil {
		return fmt.Errorf("revenue active events: %w", err)
	}
	var payRows []revenuePayDay
	if err := sourceDB.Raw(revenuePayEventsSQL, startUTC, endUTC).Scan(&payRows).Error; err != nil {
		return fmt.Errorf("revenue pay events: %w", err)
	}
	var firstRows []revenueUserDay
	if err := sourceDB.Raw(firstPayAllSQL).Scan(&firstRows).Error; err != nil {
		return fmt.Errorf("revenue first pay: %w", err)
	}
	var refundRows []revenuePayDay
	if err := sourceDB.Raw(revenueRefundEventsSQL, startUTC, endUTC, startUTC, endUTC).Scan(&refundRows).Error; err != nil {
		return fmt.Errorf("revenue refund events: %w", err)
	}
	var platformPayRows []revenuePlatformRow
	if err := sourceDB.Raw(revenuePlatformPaySQL, startUTC, endUTC).Scan(&platformPayRows).Error; err != nil {
		return fmt.Errorf("revenue platform pay: %w", err)
	}
	var platformRefundRows []revenuePlatformRefundRow
	if err := sourceDB.Raw(revenuePlatformRefundSQL, startUTC, endUTC).Scan(&platformRefundRows).Error; err != nil {
		return fmt.Errorf("revenue platform refund: %w", err)
	}
	var taxRows []revenueTaxRow
	if err := sourceDB.Raw(revenueTaxSQL, startUTC, endUTC).Scan(&taxRows).Error; err != nil {
		return fmt.Errorf("revenue tax: %w", err)
	}
	var walletRows []revenueWalletRow
	if err := sourceDB.Raw(revenueWalletSQL).Scan(&walletRows).Error; err != nil {
		return fmt.Errorf("revenue wallet balances: %w", err)
	}

	regsByUser := make(map[int64]string, len(regRows))
	regsByDay := make(map[string]map[int64]struct{})
	for _, row := range regRows {
		day := revenueDayKey(row.D)
		regsByUser[row.UserID] = day
		set := regsByDay[day]
		if set == nil {
			set = make(map[int64]struct{})
			regsByDay[day] = set
		}
		set[row.UserID] = struct{}{}
	}

	activeByDay := make(map[string]map[int64]struct{})
	for _, row := range activeRows {
		day := revenueDayKey(row.D)
		set := activeByDay[day]
		if set == nil {
			set = make(map[int64]struct{})
			activeByDay[day] = set
		}
		set[row.UserID] = struct{}{}
	}

	firstPayByUser := make(map[int64]string, len(firstRows))
	for _, row := range firstRows {
		firstPayByUser[row.UserID] = revenueDayKey(row.D)
	}

	payByDay := make(map[string]*payAgg)
	for _, row := range payRows {
		day := revenueDayKey(row.D)
		agg := payByDay[day]
		if agg == nil {
			agg = &payAgg{Users: make(map[int64]struct{}), UserAmounts: make(map[int64]int64)}
			payByDay[day] = agg
		}
		agg.Users[row.UserID] = struct{}{}
		agg.UserAmounts[row.UserID] += row.AmountFen
		agg.Amount += row.AmountFen
	}

	refundByDay := make(map[string]*refundAgg)
	for _, row := range refundRows {
		day := revenueDayKey(row.D)
		agg := refundByDay[day]
		if agg == nil {
			agg = &refundAgg{Users: make(map[int64]struct{})}
			refundByDay[day] = agg
		}
		agg.Users[row.UserID] = struct{}{}
		agg.Amount += row.AmountFen
	}

	platformByDay := make(map[string]*platformAgg)
	for _, row := range platformPayRows {
		day := revenueDayKey(row.D)
		agg := platformByDay[day]
		if agg == nil {
			agg = &platformAgg{}
			platformByDay[day] = agg
		}
		switch classifyPlatform(row.Platform) {
		case "wechat":
			agg.WechatPay += row.PayFen
			agg.WechatGift += row.GiftFen
		case "alipay":
			agg.AlipayPay += row.PayFen
			agg.AlipayGift += row.GiftFen
		case "huifu":
			agg.HuifuPay += row.PayFen
			agg.HuifuGift += row.GiftFen
		}
	}
	for _, row := range platformRefundRows {
		day := revenueDayKey(row.D)
		agg := platformByDay[day]
		if agg == nil {
			agg = &platformAgg{}
			platformByDay[day] = agg
		}
		switch classifyPlatform(row.Platform) {
		case "wechat":
			agg.WechatRefund += row.RefundFen
		case "alipay":
			agg.AlipayRefund += row.RefundFen
		case "huifu":
			agg.HuifuRefund += row.RefundFen
		}
	}

	taxByDay := make(map[string]int64, len(taxRows))
	for _, row := range taxRows {
		taxByDay[revenueDayKey(row.D)] = row.DrawFen - row.PrizeFen
	}

	var walletBalance, bonusBalance, redeemBalance, loyaltyBalance int64
	for _, row := range walletRows {
		switch row.WalletKind {
		case 1:
			walletBalance = row.Balance
		case 2:
			bonusBalance = row.Balance
		case 3:
			redeemBalance = row.Balance
		case 4:
			loyaltyBalance = row.Balance
		}
	}

	rows := make([]RevenueRow, 0, daysBetween(startLocal, endLocal)+1)
	for d := startLocal; !d.After(endLocal); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		activeSet := activeByDay[key]
		active := int64(len(activeSet))
		var newActive int64
		for userID := range activeSet {
			if regsByUser[userID] == key {
				newActive++
			}
		}

		pay := payByDay[key]
		var paying, newPaying, newPayAmount int64
		if pay != nil {
			paying = int64(len(pay.Users))
			for userID := range pay.Users {
				if firstPayByUser[userID] == key {
					newPaying++
					newPayAmount += pay.UserAmounts[userID]
				}
			}
		}
		var refundUsers, refundAmount int64
		if refund := refundByDay[key]; refund != nil {
			refundUsers = int64(len(refund.Users))
			refundAmount = refund.Amount
		}
		var payAmount int64
		if pay != nil {
			payAmount = pay.Amount
		}
		platform := platformByDay[key]
		if platform == nil {
			platform = &platformAgg{}
		}
		tax := taxByDay[key]
		var newTax int64
		if payAmount > 0 {
			newTax = tax * newPayAmount / payAmount
		}
		oldTax := tax - newTax

		churn1 := churnBps(regsByDay[key], activeByDay[d.AddDate(0, 0, 1).Format("2006-01-02")], d.AddDate(0, 0, 1), todayLocal)
		churn3 := churnBps(regsByDay[key], activeByDay[d.AddDate(0, 0, 3).Format("2006-01-02")], d.AddDate(0, 0, 3), todayLocal)

		rows = append(rows, RevenueRow{
			Date:            key,
			Registrations:   int64(len(regsByDay[key])),
			ActiveUsers:     active,
			NewActiveUsers:  newActive,
			OldActiveUsers:  active - newActive,
			ActiveUsers7d:   unionActiveCount(activeByDay, d, 6),
			ActiveUsers15d:  unionActiveCount(activeByDay, d, 14),
			Churn1dBps:      churn1,
			Churn3dBps:      churn3,
			PayAmountFen:    payAmount,
			NewPayAmountFen: newPayAmount,
			OldPayAmountFen: payAmount - newPayAmount,
			RefundAmountFen: refundAmount,
			RefundUsers:     refundUsers,
			PayingUsers:     paying,
			NewPayingUsers:  newPaying,
			OldPayingUsers:  paying - newPaying,
			NetRevenueFen:   payAmount - refundAmount,

			WechatPayFen:           platform.WechatPay,
			AlipayPayFen:           platform.AlipayPay,
			HuifuPayFen:            platform.HuifuPay,
			WechatGiftFen:          platform.WechatGift,
			AlipayGiftFen:          platform.AlipayGift,
			HuifuGiftFen:           platform.HuifuGift,
			WechatRefundFen:        platform.WechatRefund,
			AlipayRefundFen:        platform.AlipayRefund,
			HuifuRefundFen:         platform.HuifuRefund,
			TaxFen:                 tax,
			NewTaxFen:              newTax,
			OldTaxFen:              oldTax,
			WalletBalanceFen:       walletBalance,
			BonusBalanceFen:        bonusBalance,
			RedeemPointBalance:     redeemBalance,
			LoyaltyPointBalance:    loyaltyBalance,
			PlatformCurrencyPayFen: 0,
		})
	}

	return reportDB.Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			if err := upsertRevenueDaily(tx, row); err != nil {
				return err
			}
		}
		return nil
	})
}

// revenueDayKey 统一 MySQL DATE 扫描结果（部分驱动返回 RFC3339 字符串）。
func revenueDayKey(d string) string {
	if len(d) >= 10 {
		return d[:10]
	}
	return d
}

func classifyPlatform(platform string) string {
	switch {
	case strings.Contains(platform, "微信"):
		return "wechat"
	case strings.Contains(platform, "支付宝"):
		return "alipay"
	case strings.Contains(platform, "汇付"):
		return "huifu"
	default:
		return ""
	}
}

func churnBps(cohort map[int64]struct{}, active map[int64]struct{}, target, today time.Time) int64 {
	// 与留存报表一致：只统计完整自然日，目标日 >= 今天视为未到期。
	if len(cohort) == 0 || !target.Before(today) {
		return 0
	}
	var retained int64
	for userID := range cohort {
		if _, ok := active[userID]; ok {
			retained++
		}
	}
	return 10000 - rateBps(retained, int64(len(cohort)))
}

func unionActiveCount(byDay map[string]map[int64]struct{}, day time.Time, back int) int64 {
	set := make(map[int64]struct{})
	for i := 0; i <= back; i++ {
		for userID := range byDay[day.AddDate(0, 0, -i).Format("2006-01-02")] {
			set[userID] = struct{}{}
		}
	}
	return int64(len(set))
}

func upsertRevenueDaily(tx *gorm.DB, row RevenueRow) error {
	return tx.Exec(`
INSERT INTO gacha_report.revenue_daily
  (report_date, registrations, active_users, new_active_users, old_active_users,
   active_users_7d, active_users_15d, churn_1d_bps, churn_3d_bps,
   pay_amount_fen, new_pay_amount_fen, old_pay_amount_fen,
   refund_amount_fen, refund_users, paying_users,
   new_paying_users, old_paying_users, net_revenue_fen,
   wechat_pay_fen, alipay_pay_fen, huifu_pay_fen,
   wechat_gift_fen, alipay_gift_fen, huifu_gift_fen,
   wechat_refund_fen, alipay_refund_fen, huifu_refund_fen,
   wechat_gift_refund_fen, alipay_gift_refund_fen, huifu_gift_refund_fen,
   tax_fen, new_tax_fen, old_tax_fen,
   wallet_balance_fen, bonus_balance_fen, redeem_point_balance, loyalty_point_balance,
   platform_currency_pay_fen)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  registrations = VALUES(registrations),
  active_users = VALUES(active_users),
  new_active_users = VALUES(new_active_users),
  old_active_users = VALUES(old_active_users),
  active_users_7d = VALUES(active_users_7d),
  active_users_15d = VALUES(active_users_15d),
  churn_1d_bps = VALUES(churn_1d_bps),
  churn_3d_bps = VALUES(churn_3d_bps),
  pay_amount_fen = VALUES(pay_amount_fen),
  new_pay_amount_fen = VALUES(new_pay_amount_fen),
  old_pay_amount_fen = VALUES(old_pay_amount_fen),
  refund_amount_fen = VALUES(refund_amount_fen),
  refund_users = VALUES(refund_users),
  paying_users = VALUES(paying_users),
  new_paying_users = VALUES(new_paying_users),
  old_paying_users = VALUES(old_paying_users),
  net_revenue_fen = VALUES(net_revenue_fen),
  wechat_pay_fen = VALUES(wechat_pay_fen),
  alipay_pay_fen = VALUES(alipay_pay_fen),
  huifu_pay_fen = VALUES(huifu_pay_fen),
  wechat_gift_fen = VALUES(wechat_gift_fen),
  alipay_gift_fen = VALUES(alipay_gift_fen),
  huifu_gift_fen = VALUES(huifu_gift_fen),
  wechat_refund_fen = VALUES(wechat_refund_fen),
  alipay_refund_fen = VALUES(alipay_refund_fen),
  huifu_refund_fen = VALUES(huifu_refund_fen),
  wechat_gift_refund_fen = VALUES(wechat_gift_refund_fen),
  alipay_gift_refund_fen = VALUES(alipay_gift_refund_fen),
  huifu_gift_refund_fen = VALUES(huifu_gift_refund_fen),
  tax_fen = VALUES(tax_fen),
  new_tax_fen = VALUES(new_tax_fen),
  old_tax_fen = VALUES(old_tax_fen),
  wallet_balance_fen = VALUES(wallet_balance_fen),
  bonus_balance_fen = VALUES(bonus_balance_fen),
  redeem_point_balance = VALUES(redeem_point_balance),
  loyalty_point_balance = VALUES(loyalty_point_balance),
  platform_currency_pay_fen = VALUES(platform_currency_pay_fen),
  updated_at = NOW(3)`,
		row.Date, row.Registrations, row.ActiveUsers, row.NewActiveUsers, row.OldActiveUsers,
		row.ActiveUsers7d, row.ActiveUsers15d, row.Churn1dBps, row.Churn3dBps,
		row.PayAmountFen, row.NewPayAmountFen, row.OldPayAmountFen,
		row.RefundAmountFen, row.RefundUsers, row.PayingUsers,
		row.NewPayingUsers, row.OldPayingUsers, row.NetRevenueFen,
		row.WechatPayFen, row.AlipayPayFen, row.HuifuPayFen,
		row.WechatGiftFen, row.AlipayGiftFen, row.HuifuGiftFen,
		row.WechatRefundFen, row.AlipayRefundFen, row.HuifuRefundFen,
		row.WechatGiftRefundFen, row.AlipayGiftRefundFen, row.HuifuGiftRefundFen,
		row.TaxFen, row.NewTaxFen, row.OldTaxFen,
		row.WalletBalanceFen, row.BonusBalanceFen, row.RedeemPointBalance, row.LoyaltyPointBalance,
		row.PlatformCurrencyPayFen,
	).Error
}

// StartRevenue 启动营收日报聚合：启动回填，之后每 10 分钟刷新近 180 天。
func StartRevenue(sourceDB, reportDB *gorm.DB) {
	if sourceDB == nil || reportDB == nil {
		return
	}
	go func() {
		today := time.Now().In(cst)
		start := today.AddDate(0, 0, -400)
		if earliest, ok := earliestBusinessDate(sourceDB); ok && earliest.After(start) {
			start = earliest
		}
		runRevenueAggregation(sourceDB, reportDB, start, today)

		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now().In(cst)
			start := now.AddDate(0, 0, -180)
			runRevenueAggregation(sourceDB, reportDB, start, now)
		}
	}()
}

func runRevenueAggregation(sourceDB, reportDB *gorm.DB, start, end time.Time) {
	ctx := context.Background()
	sqlDB, err := reportDB.DB()
	if err != nil {
		logx.Errorf("revenue aggregate: get report sql.DB failed: %v", err)
		return
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		logx.Errorf("revenue aggregate: acquire lock connection failed: %v", err)
		return
	}
	defer conn.Close()

	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", "gacha_report_revenue").Scan(&got); err != nil {
		logx.Errorf("revenue aggregate: acquire lock failed: %v", err)
		return
	}
	if !got.Valid || got.Int64 != 1 {
		logx.Infof("revenue aggregate skipped (lock busy), start: %s, end: %s",
			start.Format("2006-01-02"), end.Format("2006-01-02"))
		return
	}
	defer func() {
		var released sql.NullInt64
		_ = conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", "gacha_report_revenue").Scan(&released)
	}()

	if err := AggregateRevenue(sourceDB, reportDB, start, end); err != nil {
		logx.Errorf("revenue aggregate failed, start: %s, end: %s, err: %v",
			start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return
	}
	logx.Infof("revenue aggregate done, start: %s, end: %s",
		start.Format("2006-01-02"), end.Format("2006-01-02"))
}
