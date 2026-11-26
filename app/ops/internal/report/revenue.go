package report

import (
	"context"
	"database/sql"
	"fmt"
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
       new_paying_users, old_paying_users, net_revenue_fen
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
		}
		if row.PayingUsers > 0 {
			row.ArppuFen = row.PayAmountFen / row.PayingUsers
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
   new_paying_users, old_paying_users, net_revenue_fen)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
  updated_at = NOW(3)`,
		row.Date, row.Registrations, row.ActiveUsers, row.NewActiveUsers, row.OldActiveUsers,
		row.ActiveUsers7d, row.ActiveUsers15d, row.Churn1dBps, row.Churn3dBps,
		row.PayAmountFen, row.NewPayAmountFen, row.OldPayAmountFen,
		row.RefundAmountFen, row.RefundUsers, row.PayingUsers,
		row.NewPayingUsers, row.OldPayingUsers, row.NetRevenueFen,
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
