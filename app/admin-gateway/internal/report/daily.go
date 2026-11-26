// Package report 经营日报只读聚合。
// 数据跨 identity/money/draw/catalog 四个库，SQL 全部使用只读聚合查询。
package report

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

var cst = time.FixedZone("CST", 8*3600)

// Summary 累计/本月汇总（金额单位：分，比率单位：万分比）。
type Summary struct {
	Registrations             int64 `json:"registrations"`
	PayingUsers               int64 `json:"payingUsers"`
	DrawCount                 int64 `json:"drawCount"`
	DrawAmountFen             int64 `json:"drawAmountFen"`
	PrizeAmountFen            int64 `json:"prizeAmountFen"`
	GrossProfitFen            int64 `json:"grossProfitFen"`
	GrossMarginBps            int64 `json:"grossMarginBps"`
	TotalReceivedFen          int64 `json:"totalReceivedFen"`
	TotalRechargeFen          int64 `json:"totalRechargeFen"`
	TotalRefundFen            int64 `json:"totalRefundFen"`
	AlipayRechargeFen         int64 `json:"alipayRechargeFen"`
	HuifuRechargeFen          int64 `json:"huifuRechargeFen"`
	AlipayRefundFen           int64 `json:"alipayRefundFen"`
	HuifuRefundFen            int64 `json:"huifuRefundFen"`
	RechargeOriginalRefundFen int64 `json:"rechargeOriginalRefundFen"`
	UserRefundFen             int64 `json:"userRefundFen"`
	RechargeMinusRefundFen    int64 `json:"rechargeMinusRefundFen"`
	RefundRateBps             int64 `json:"refundRateBps"`
}

// Row 每日明细（金额单位：分，比率单位：万分比）。
type Row struct {
	Date                      string `json:"date"`
	Registrations             int64  `json:"registrations"`
	ActiveUsers               int64  `json:"activeUsers"`
	PayingUsers               int64  `json:"payingUsers"`
	RefundableFen             int64  `json:"refundableFen"`
	BalanceFen                int64  `json:"balanceFen"`
	InventoryValueFen         int64  `json:"inventoryValueFen"`
	DrawAmountFen             int64  `json:"drawAmountFen"`
	PrizeAmountFen            int64  `json:"prizeAmountFen"`
	GrossProfitFen            int64  `json:"grossProfitFen"`
	GrossMarginBps            int64  `json:"grossMarginBps"`
	TotalReceivedFen          int64  `json:"totalReceivedFen"`
	TotalRechargeFen          int64  `json:"totalRechargeFen"`
	TotalRefundFen            int64  `json:"totalRefundFen"`
	AlipayRechargeFen         int64  `json:"alipayRechargeFen"`
	HuifuRechargeFen          int64  `json:"huifuRechargeFen"`
	AlipayRefundFen           int64  `json:"alipayRefundFen"`
	HuifuRefundFen            int64  `json:"huifuRefundFen"`
	RechargeOriginalRefundFen int64  `json:"rechargeOriginalRefundFen"`
	UserRefundFen             int64  `json:"userRefundFen"`
	RechargeMinusRefundFen    int64  `json:"rechargeMinusRefundFen"`
	RefundRateBps             int64  `json:"refundRateBps"`
}

type Result struct {
	Summary Summary `json:"summary"`
	Month   Summary `json:"month"`
	Rows    []Row   `json:"rows"`
}

type countRow struct {
	D string `gorm:"column:d"`
	N int64  `gorm:"column:n"`
}

type drawRow struct {
	D        string `gorm:"column:d"`
	Draws    int64  `gorm:"column:draws"`
	DrawFen  int64  `gorm:"column:draw_fen"`
	PrizeFen int64  `gorm:"column:prize_fen"`
}

type rechargeRow struct {
	D           string `gorm:"column:d"`
	PayingUsers int64  `gorm:"column:paying_users"`
	RechargeFen int64  `gorm:"column:recharge_fen"`
	ReceivedFen int64  `gorm:"column:received_fen"`
	AlipayFen   int64  `gorm:"column:alipay_fen"`
	HuifuFen    int64  `gorm:"column:huifu_fen"`
}

type rechargeRefundRow struct {
	D         string `gorm:"column:d"`
	RefundFen int64  `gorm:"column:refund_fen"`
	AlipayFen int64  `gorm:"column:alipay_fen"`
	HuifuFen  int64  `gorm:"column:huifu_fen"`
}

type refundRow struct {
	D         string `gorm:"column:d"`
	RefundFen int64  `gorm:"column:refund_fen"`
}

// QueryDaily 查询 [start, end]（本地日期，闭区间）的经营日报。
func QueryDaily(db *gorm.DB, start, end time.Time) (*Result, error) {
	startLocal := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, cst)
	endLocal := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, cst)
	if endLocal.Before(startLocal) {
		return nil, fmt.Errorf("end date before start date")
	}
	startUTC := startLocal.UTC()
	endUTC := endLocal.AddDate(0, 0, 1).UTC()
	allStart := time.Date(2000, 1, 1, 0, 0, 0, 0, cst).UTC()
	monthStart := time.Date(endLocal.Year(), endLocal.Month(), 1, 0, 0, 0, 0, cst).UTC()

	summary, err := loadSummary(db, allStart, endUTC)
	if err != nil {
		return nil, err
	}
	month, err := loadSummary(db, monthStart, endUTC)
	if err != nil {
		return nil, err
	}
	rows, err := loadRows(db, startLocal, endLocal, startUTC, endUTC)
	if err != nil {
		return nil, err
	}
	return &Result{Summary: *summary, Month: *month, Rows: rows}, nil
}

func loadSummary(db *gorm.DB, startUTC, endUTC time.Time) (*Summary, error) {
	var s Summary

	if err := db.Raw(
		"SELECT COUNT(*) FROM gacha_identity.user_users WHERE created_at >= ? AND created_at < ?",
		startUTC, endUTC,
	).Scan(&s.Registrations).Error; err != nil {
		return nil, fmt.Errorf("summary registrations: %w", err)
	}
	if err := db.Raw(
		"SELECT COUNT(DISTINCT user_id) FROM gacha_money.recharge_orders WHERE status = 4 AND created_at >= ? AND created_at < ?",
		startUTC, endUTC,
	).Scan(&s.PayingUsers).Error; err != nil {
		return nil, fmt.Errorf("summary paying users: %w", err)
	}

	var draw struct {
		N        int64
		DrawFen  int64
		PrizeFen int64
	}
	if err := db.Raw(`
SELECT COUNT(*) AS n,
       COALESCE(SUM(r.price), 0) AS draw_fen,
       COALESCE(SUM(CASE WHEN r.is_prize = 1 THEN COALESCE(e.display_value_fen, 0) ELSE 0 END), 0) AS prize_fen
FROM gacha_draw.draw_records r
LEFT JOIN gacha_draw.draw_pool_entries e ON e.id = r.entry_id
WHERE r.status = 1 AND r.created_at >= ? AND r.created_at < ?`,
		startUTC, endUTC,
	).Scan(&draw).Error; err != nil {
		return nil, fmt.Errorf("summary draw: %w", err)
	}
	s.DrawCount = draw.N
	s.DrawAmountFen = draw.DrawFen
	s.PrizeAmountFen = draw.PrizeFen

	var recharge struct {
		RechargeFen int64
		ReceivedFen int64
		AlipayFen   int64
		HuifuFen    int64
	}
	if err := db.Raw(`
SELECT COALESCE(SUM(amount_fen), 0) AS recharge_fen,
       COALESCE(SUM(credited_fen), 0) AS received_fen,
       COALESCE(SUM(CASE WHEN platform = '支付宝' THEN amount_fen ELSE 0 END), 0) AS alipay_fen,
       COALESCE(SUM(CASE WHEN platform = '汇付' THEN amount_fen ELSE 0 END), 0) AS huifu_fen
FROM gacha_money.recharge_orders
WHERE status = 4 AND created_at >= ? AND created_at < ?`,
		startUTC, endUTC,
	).Scan(&recharge).Error; err != nil {
		return nil, fmt.Errorf("summary recharge: %w", err)
	}
	s.TotalRechargeFen = recharge.RechargeFen
	s.TotalReceivedFen = recharge.ReceivedFen
	s.AlipayRechargeFen = recharge.AlipayFen
	s.HuifuRechargeFen = recharge.HuifuFen

	var rechargeRefund struct {
		RefundFen int64
		AlipayFen int64
		HuifuFen  int64
	}
	if err := db.Raw(`
SELECT COALESCE(SUM(refund_amount_fen), 0) AS refund_fen,
       COALESCE(SUM(CASE WHEN platform = '支付宝' THEN refund_amount_fen ELSE 0 END), 0) AS alipay_fen,
       COALESCE(SUM(CASE WHEN platform = '汇付' THEN refund_amount_fen ELSE 0 END), 0) AS huifu_fen
FROM gacha_money.recharge_orders
WHERE refund_amount_fen > 0 AND refunded_at >= ? AND refunded_at < ?`,
		startUTC, endUTC,
	).Scan(&rechargeRefund).Error; err != nil {
		return nil, fmt.Errorf("summary recharge refund: %w", err)
	}
	s.RechargeOriginalRefundFen = rechargeRefund.RefundFen
	s.AlipayRefundFen = rechargeRefund.AlipayFen
	s.HuifuRefundFen = rechargeRefund.HuifuFen

	if err := db.Raw(
		"SELECT COALESCE(SUM(amount_fen), 0) FROM gacha_money.refund_orders WHERE status = 3 AND completed_at >= ? AND completed_at < ?",
		startUTC, endUTC,
	).Scan(&s.UserRefundFen).Error; err != nil {
		return nil, fmt.Errorf("summary user refund: %w", err)
	}

	s.TotalRefundFen = s.RechargeOriginalRefundFen + s.UserRefundFen
	s.GrossProfitFen = s.DrawAmountFen - s.PrizeAmountFen
	s.GrossMarginBps = rateBps(s.GrossProfitFen, s.DrawAmountFen)
	s.RechargeMinusRefundFen = s.TotalRechargeFen - s.TotalRefundFen
	s.RefundRateBps = rateBps(s.TotalRefundFen, s.TotalRechargeFen)
	return &s, nil
}

func loadRows(db *gorm.DB, startLocal, endLocal, startUTC, endUTC time.Time) ([]Row, error) {
	regs, err := loadCounts(db, "gacha_identity.user_users", startUTC, endUTC)
	if err != nil {
		return nil, err
	}
	actives, err := loadActiveCounts(db, startUTC, endUTC)
	if err != nil {
		return nil, err
	}
	draws, err := loadDraws(db, startUTC, endUTC)
	if err != nil {
		return nil, err
	}
	recharges, err := loadRecharges(db, startUTC, endUTC)
	if err != nil {
		return nil, err
	}
	rechargeRefunds, err := loadRechargeRefunds(db, startUTC, endUTC)
	if err != nil {
		return nil, err
	}
	refunds, err := loadRefunds(db, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	currentInventory, err := currentInventoryValue(db)
	if err != nil {
		return nil, err
	}
	currentRefundable, err := currentRefundableValue(db)
	if err != nil {
		return nil, err
	}
	currentBalance, err := currentCashBalance(db)
	if err != nil {
		return nil, err
	}
	futurePrize, err := futurePrizeByDay(db, startUTC)
	if err != nil {
		return nil, err
	}

	rows := make([]Row, 0, daysBetween(startLocal, endLocal)+1)
	for d := startLocal; !d.After(endLocal); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		draw := draws[key]
		recharge := recharges[key]
		rechargeRefund := rechargeRefunds[key]
		userRefund := refunds[key]

		totalRefund := rechargeRefund.RefundFen + userRefund.RefundFen
		grossProfit := draw.DrawFen - draw.PrizeFen
		inventory := currentInventory + futurePrizeAfter(futurePrize, key)

		rows = append(rows, Row{
			Date:                      key,
			Registrations:             regs[key],
			ActiveUsers:               actives[key],
			PayingUsers:               recharge.PayingUsers,
			RefundableFen:             currentRefundable,
			BalanceFen:                currentBalance,
			InventoryValueFen:         inventory,
			DrawAmountFen:             draw.DrawFen,
			PrizeAmountFen:            draw.PrizeFen,
			GrossProfitFen:            grossProfit,
			GrossMarginBps:            rateBps(grossProfit, draw.DrawFen),
			TotalReceivedFen:          recharge.ReceivedFen,
			TotalRechargeFen:          recharge.RechargeFen,
			TotalRefundFen:            totalRefund,
			AlipayRechargeFen:         recharge.AlipayFen,
			HuifuRechargeFen:          recharge.HuifuFen,
			AlipayRefundFen:           rechargeRefund.AlipayFen,
			HuifuRefundFen:            rechargeRefund.HuifuFen,
			RechargeOriginalRefundFen: rechargeRefund.RefundFen,
			UserRefundFen:             userRefund.RefundFen,
			RechargeMinusRefundFen:    recharge.RechargeFen - totalRefund,
			RefundRateBps:             rateBps(totalRefund, recharge.RechargeFen),
		})
	}
	// 与后台其他列表一致：日期倒序，最新一天在最上面。
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

func loadCounts(db *gorm.DB, table string, startUTC, endUTC time.Time) (map[string]int64, error) {
	var rows []countRow
	if err := db.Raw(fmt.Sprintf(
		"SELECT DATE_FORMAT(DATE_ADD(created_at, INTERVAL 8 HOUR), '%%Y-%%m-%%d') AS d, COUNT(*) AS n FROM %s WHERE created_at >= ? AND created_at < ? GROUP BY d",
		table,
	), startUTC, endUTC).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("daily counts %s: %w", table, err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.D] = r.N
	}
	return out, nil
}

func loadActiveCounts(db *gorm.DB, startUTC, endUTC time.Time) (map[string]int64, error) {
	var rows []countRow
	if err := db.Raw(`
SELECT d, COUNT(DISTINCT user_id) AS n FROM (
  SELECT DATE_FORMAT(DATE_ADD(created_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS d, user_id
  FROM gacha_draw.draw_records WHERE created_at >= ? AND created_at < ?
  UNION ALL
  SELECT DATE_FORMAT(DATE_ADD(created_at, INTERVAL 8 HOUR), '%Y-%m-%d'), user_id
  FROM gacha_money.recharge_orders WHERE created_at >= ? AND created_at < ?
  UNION ALL
  SELECT DATE_FORMAT(DATE_ADD(created_at, INTERVAL 8 HOUR), '%Y-%m-%d'), user_id
  FROM gacha_money.refund_orders WHERE created_at >= ? AND created_at < ?
  UNION ALL
  SELECT DATE_FORMAT(DATE_ADD(created_at, INTERVAL 8 HOUR), '%Y-%m-%d'), user_id
  FROM gacha_money.payment_orders WHERE created_at >= ? AND created_at < ?
) x GROUP BY d`,
		startUTC, endUTC, startUTC, endUTC, startUTC, endUTC, startUTC, endUTC,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("daily active users: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.D] = r.N
	}
	return out, nil
}

func loadDraws(db *gorm.DB, startUTC, endUTC time.Time) (map[string]drawRow, error) {
	var rows []drawRow
	if err := db.Raw(`
SELECT DATE_FORMAT(DATE_ADD(r.created_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS d,
       COUNT(*) AS draws,
       COALESCE(SUM(r.price), 0) AS draw_fen,
       COALESCE(SUM(CASE WHEN r.is_prize = 1 THEN COALESCE(e.display_value_fen, 0) ELSE 0 END), 0) AS prize_fen
FROM gacha_draw.draw_records r
LEFT JOIN gacha_draw.draw_pool_entries e ON e.id = r.entry_id
WHERE r.status = 1 AND r.created_at >= ? AND r.created_at < ?
GROUP BY d`,
		startUTC, endUTC,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("daily draw: %w", err)
	}
	out := make(map[string]drawRow, len(rows))
	for _, r := range rows {
		out[r.D] = r
	}
	return out, nil
}

func loadRecharges(db *gorm.DB, startUTC, endUTC time.Time) (map[string]rechargeRow, error) {
	var rows []rechargeRow
	if err := db.Raw(`
SELECT DATE_FORMAT(DATE_ADD(created_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS d,
       COUNT(DISTINCT user_id) AS paying_users,
       COALESCE(SUM(amount_fen), 0) AS recharge_fen,
       COALESCE(SUM(credited_fen), 0) AS received_fen,
       COALESCE(SUM(CASE WHEN platform = '支付宝' THEN amount_fen ELSE 0 END), 0) AS alipay_fen,
       COALESCE(SUM(CASE WHEN platform = '汇付' THEN amount_fen ELSE 0 END), 0) AS huifu_fen
FROM gacha_money.recharge_orders
WHERE status = 4 AND created_at >= ? AND created_at < ?
GROUP BY d`,
		startUTC, endUTC,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("daily recharge: %w", err)
	}
	out := make(map[string]rechargeRow, len(rows))
	for _, r := range rows {
		out[r.D] = r
	}
	return out, nil
}

func loadRechargeRefunds(db *gorm.DB, startUTC, endUTC time.Time) (map[string]rechargeRefundRow, error) {
	var rows []rechargeRefundRow
	if err := db.Raw(`
SELECT DATE_FORMAT(DATE_ADD(refunded_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS d,
       COALESCE(SUM(refund_amount_fen), 0) AS refund_fen,
       COALESCE(SUM(CASE WHEN platform = '支付宝' THEN refund_amount_fen ELSE 0 END), 0) AS alipay_fen,
       COALESCE(SUM(CASE WHEN platform = '汇付' THEN refund_amount_fen ELSE 0 END), 0) AS huifu_fen
FROM gacha_money.recharge_orders
WHERE refund_amount_fen > 0 AND refunded_at >= ? AND refunded_at < ?
GROUP BY d`,
		startUTC, endUTC,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("daily recharge refund: %w", err)
	}
	out := make(map[string]rechargeRefundRow, len(rows))
	for _, r := range rows {
		out[r.D] = r
	}
	return out, nil
}

func loadRefunds(db *gorm.DB, startUTC, endUTC time.Time) (map[string]refundRow, error) {
	var rows []refundRow
	if err := db.Raw(`
SELECT DATE_FORMAT(DATE_ADD(completed_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS d,
       COALESCE(SUM(amount_fen), 0) AS refund_fen
FROM gacha_money.refund_orders
WHERE status = 3 AND completed_at >= ? AND completed_at < ?
GROUP BY d`,
		startUTC, endUTC,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("daily user refund: %w", err)
	}
	out := make(map[string]refundRow, len(rows))
	for _, r := range rows {
		out[r.D] = r
	}
	return out, nil
}

func currentInventoryValue(db *gorm.DB) (int64, error) {
	var v int64
	err := db.Raw(`
SELECT COALESCE(SUM(GREATEST(remaining, 0) * COALESCE(display_value_fen, 0)), 0)
FROM gacha_draw.draw_pool_entries
WHERE is_prize = 1 AND quantity <> -1 AND remaining > 0`).Scan(&v).Error
	if err != nil {
		return 0, fmt.Errorf("current inventory value: %w", err)
	}
	return v, nil
}

func currentRefundableValue(db *gorm.DB) (int64, error) {
	var v int64
	err := db.Raw(`
SELECT COALESCE(SUM(COALESCE(e.display_value_fen, 0)), 0)
FROM gacha_draw.draw_warehouse_items w
JOIN gacha_draw.draw_records r ON r.id = w.record_id
LEFT JOIN gacha_draw.draw_pool_entries e ON e.id = r.entry_id
WHERE w.status = 1`).Scan(&v).Error
	if err != nil {
		return 0, fmt.Errorf("current refundable value: %w", err)
	}
	return v, nil
}

func currentCashBalance(db *gorm.DB) (int64, error) {
	var v int64
	err := db.Raw(`
SELECT COALESCE(SUM(available + held), 0)
FROM gacha_money.wallet_accounts
WHERE wallet_kind = 1 AND currency = 'CNY'`).Scan(&v).Error
	if err != nil {
		return 0, fmt.Errorf("current cash balance: %w", err)
	}
	return v, nil
}

func futurePrizeByDay(db *gorm.DB, startUTC time.Time) (map[string]int64, error) {
	var rows []struct {
		D string `gorm:"column:d"`
		V int64  `gorm:"column:v"`
	}
	if err := db.Raw(`
SELECT DATE_FORMAT(DATE_ADD(r.created_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS d,
       COALESCE(SUM(COALESCE(e.display_value_fen, 0)), 0) AS v
FROM gacha_draw.draw_records r
JOIN gacha_draw.draw_pool_entries e ON e.id = r.entry_id
WHERE r.status = 1 AND r.is_prize = 1 AND e.quantity <> -1 AND r.created_at >= ?
GROUP BY d`,
		startUTC,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("future prize value: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.D] = r.V
	}
	return out, nil
}

// futurePrizeAfter 返回严格晚于该日期的奖品价值合计（用于倒推库存价值）。
func futurePrizeAfter(byDay map[string]int64, day string) int64 {
	var total int64
	for d, v := range byDay {
		if d > day {
			total += v
		}
	}
	return total
}

func rateBps(part, total int64) int64 {
	if total <= 0 {
		return 0
	}
	return part * 10000 / total
}

func daysBetween(start, end time.Time) int {
	return int(end.Sub(start).Hours() / 24)
}
