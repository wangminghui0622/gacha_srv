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

// 留存报表类型：与 gacha_report 中 5 张读模型表一一对应。
const (
	RetentionTypeNew    = "new"     // 新增留存
	RetentionTypeActive = "active"  // 有效用户
	RetentionTypeNewPay = "new_pay" // 新增消费
	RetentionTypeJoin   = "join"    // 参与留存
	RetentionTypePay    = "pay"     // 消费留存
)

// RetentionDays 留存报表固定展示的 D1..D180。
var RetentionDays = []int{1, 2, 3, 4, 5, 6, 7, 14, 30, 60, 90, 180}

var retentionTables = map[string]string{
	RetentionTypeNew:    "retention_new",
	RetentionTypeActive: "retention_active",
	RetentionTypeNewPay: "retention_new_pay",
	RetentionTypeJoin:   "retention_join",
	RetentionTypePay:    "retention_pay",
}

// IsRetentionType 判断类型是否受支持。
func IsRetentionType(t string) bool {
	_, ok := retentionTables[t]
	return ok
}

// ── 业务事件 SQL（统一活跃口径）──
//
// 活跃 = 有效抽赏 / 成功充值 / 成功支付 / 成功退款，任一业务行为。
// 时间字段统一按 UTC 存储，展示日期按东八区 +8h 取自然日。
const activeEventsSQL = `
SELECT user_id, DATE(DATE_ADD(created_at, INTERVAL 8 HOUR)) AS d
FROM gacha_draw.draw_records
WHERE status = 1 AND created_at >= ? AND created_at < ?
UNION
SELECT user_id, DATE(DATE_ADD(COALESCE(completed_at, created_at), INTERVAL 8 HOUR)) AS d
FROM gacha_money.recharge_orders
WHERE status = 4 AND COALESCE(completed_at, created_at) >= ? AND COALESCE(completed_at, created_at) < ?
UNION
SELECT user_id, DATE(DATE_ADD(created_at, INTERVAL 8 HOUR)) AS d
FROM gacha_money.payment_orders
WHERE status = 3 AND created_at >= ? AND created_at < ?
UNION
SELECT user_id, DATE(DATE_ADD(COALESCE(completed_at, created_at), INTERVAL 8 HOUR)) AS d
FROM gacha_money.refund_orders
WHERE status = 3 AND COALESCE(completed_at, created_at) >= ? AND COALESCE(completed_at, created_at) < ?`

// drawEventsSQL 参与行为：有效抽赏。
const drawEventsSQL = `
SELECT DISTINCT user_id, DATE(DATE_ADD(created_at, INTERVAL 8 HOUR)) AS d
FROM gacha_draw.draw_records
WHERE status = 1 AND created_at >= ? AND created_at < ?`

// rechargeEventsSQL 消费行为：成功充值（完成时间为准，缺失时回退创建时间）。
const rechargeEventsSQL = `
SELECT DISTINCT user_id, DATE(DATE_ADD(COALESCE(completed_at, created_at), INTERVAL 8 HOUR)) AS d
FROM gacha_money.recharge_orders
WHERE status = 4 AND COALESCE(completed_at, created_at) >= ? AND COALESCE(completed_at, created_at) < ?`

// registrationEventsSQL 注册 cohort（user_users 主键为 id，统一别名 user_id）。
const registrationEventsSQL = `
SELECT id AS user_id, DATE(DATE_ADD(created_at, INTERVAL 8 HOUR)) AS d
FROM gacha_identity.user_users
WHERE created_at >= ? AND created_at < ?`

// firstPayEventsSQL 首次成功充值 cohort（全历史首充，避免把老付费用户算成新客）。
const firstPayEventsSQL = `
SELECT user_id, d FROM (
  SELECT user_id, DATE(DATE_ADD(MIN(COALESCE(completed_at, created_at)), INTERVAL 8 HOUR)) AS d
  FROM gacha_money.recharge_orders
  WHERE status = 4
  GROUP BY user_id
) x WHERE d >= ? AND d <= ?`

// RetentionCell 单个留存天数单元格（total_users 为该列可用 cohort 的分母）。
type RetentionCell struct {
	DayNo         int   `json:"dayNo"`
	TotalUsers    int64 `json:"totalUsers"`
	RetainedUsers int64 `json:"retainedUsers"`
	RateBps       int64 `json:"rateBps"`
	Available     bool  `json:"available"`
}

// RetentionRow 一个 cohort 日期（或 TOTAL 汇总行）。
type RetentionRow struct {
	CohortDate string          `json:"cohortDate"`
	TotalUsers int64           `json:"totalUsers"`
	Cells      []RetentionCell `json:"cells"`
}

// RetentionResult 单张留存报表。
type RetentionResult struct {
	Type   string         `json:"type"`
	DayNos []int          `json:"dayNos"`
	Rows   []RetentionRow `json:"rows"`
	Total  RetentionRow   `json:"total"`
}

type retentionDBRow struct {
	CohortDate    string `gorm:"column:cohort_date"`
	DayNo         int    `gorm:"column:day_no"`
	TotalUsers    int64  `gorm:"column:total_users"`
	RetainedUsers int64  `gorm:"column:retained_users"`
}

type retentionAggRow struct {
	CohortDate    string `gorm:"column:cohort_date"`
	DayNo         int    `gorm:"column:day_no"`
	TotalUsers    int64  `gorm:"column:total_users"`
	RetainedUsers int64  `gorm:"column:retained_users"`
}

// QueryRetention 从 gacha_report 读模型查询 [start, end] 的留存报表。
// 返回行按 cohort_date 倒序（与经营日报一致），TOTAL 行由服务端汇总。
func QueryRetention(reportDB *gorm.DB, typ string, start, end time.Time) (*RetentionResult, error) {
	table, ok := retentionTables[typ]
	if !ok {
		return nil, fmt.Errorf("unknown retention type %q", typ)
	}
	startLocal := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, cst)
	endLocal := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, cst)
	if endLocal.Before(startLocal) {
		return nil, fmt.Errorf("end date before start date")
	}

	var dbRows []retentionDBRow
	if err := reportDB.Raw(fmt.Sprintf(`
SELECT DATE_FORMAT(cohort_date, '%%Y-%%m-%%d') AS cohort_date,
       day_no, total_users, retained_users
FROM gacha_report.%s
WHERE cohort_date >= ? AND cohort_date <= ?
ORDER BY cohort_date, day_no`, table),
		startLocal.Format("2006-01-02"), endLocal.Format("2006-01-02"),
	).Scan(&dbRows).Error; err != nil {
		return nil, fmt.Errorf("query %s: %w", table, err)
	}

	type dayAgg struct {
		total int64
		cells map[int]retentionDBRow
	}
	byDate := make(map[string]*dayAgg, len(dbRows))
	for _, row := range dbRows {
		agg := byDate[row.CohortDate]
		if agg == nil {
			agg = &dayAgg{cells: make(map[int]retentionDBRow)}
			byDate[row.CohortDate] = agg
		}
		if row.DayNo == 0 {
			agg.total = row.TotalUsers
			continue
		}
		agg.cells[row.DayNo] = row
	}

	result := &RetentionResult{
		Type:   typ,
		DayNos: append([]int(nil), RetentionDays...),
		Rows:   make([]RetentionRow, 0, daysBetween(startLocal, endLocal)+1),
	}
	for d := endLocal; !d.Before(startLocal); d = d.AddDate(0, 0, -1) {
		key := d.Format("2006-01-02")
		agg := byDate[key]
		row := RetentionRow{
			CohortDate: key,
			Cells:      make([]RetentionCell, 0, len(RetentionDays)),
		}
		if agg != nil {
			row.TotalUsers = agg.total
		}
		for _, dayNo := range RetentionDays {
			cell := RetentionCell{DayNo: dayNo, TotalUsers: row.TotalUsers}
			if agg != nil {
				if rec, ok := agg.cells[dayNo]; ok && row.TotalUsers > 0 {
					cell.Available = true
					cell.RetainedUsers = rec.RetainedUsers
					cell.RateBps = rateBps(rec.RetainedUsers, row.TotalUsers)
				}
			}
			row.Cells = append(row.Cells, cell)
		}
		result.Rows = append(result.Rows, row)
	}

	total := RetentionRow{
		CohortDate: "TOTAL",
		Cells:      make([]RetentionCell, 0, len(RetentionDays)),
	}
	for _, row := range result.Rows {
		total.TotalUsers += row.TotalUsers
	}
	for i, dayNo := range RetentionDays {
		var denominator, retained int64
		for _, row := range result.Rows {
			cell := row.Cells[i]
			if !cell.Available {
				continue
			}
			denominator += row.TotalUsers
			retained += cell.RetainedUsers
		}
		total.Cells = append(total.Cells, RetentionCell{
			DayNo:         dayNo,
			TotalUsers:    denominator,
			RetainedUsers: retained,
			RateBps:       rateBps(retained, denominator),
			Available:     denominator > 0,
		})
	}
	result.Total = total
	return result, nil
}

// AggregateRetention 从业务库聚合 [start, end] 的 5 张留存表，写入 gacha_report。
func AggregateRetention(sourceDB, reportDB *gorm.DB, start, end time.Time) error {
	startLocal := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, cst)
	endLocal := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, cst)
	today := time.Now().In(cst)
	todayLocal := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, cst)
	if endLocal.Before(startLocal) {
		return fmt.Errorf("retention end date before start date")
	}
	if endLocal.After(todayLocal) {
		endLocal = todayLocal
	}
	if endLocal.Before(startLocal) {
		return nil
	}

	startUTC := startLocal.UTC()
	endUTC := endLocal.AddDate(0, 0, 1).UTC()
	todayUTC := todayLocal.AddDate(0, 0, 1).UTC()
	dueDate := todayLocal.Format("2006-01-02")

	activeArgs := func(from, to time.Time) []any {
		return []any{from, to, from, to, from, to, from, to}
	}

	type spec struct {
		table        string
		cohortSQL    string
		cohortArgs   []any
		retainedSQL  string
		retainedArgs []any
	}
	specs := []spec{
		{
			table:        retentionTables[RetentionTypeNew],
			cohortSQL:    registrationEventsSQL,
			cohortArgs:   []any{startUTC, endUTC},
			retainedSQL:  activeEventsSQL,
			retainedArgs: activeArgs(startUTC, todayUTC),
		},
		{
			table:        retentionTables[RetentionTypeActive],
			cohortSQL:    activeEventsSQL,
			cohortArgs:   activeArgs(startUTC, endUTC),
			retainedSQL:  activeEventsSQL,
			retainedArgs: activeArgs(startUTC, todayUTC),
		},
		{
			table:        retentionTables[RetentionTypeNewPay],
			cohortSQL:    firstPayEventsSQL,
			cohortArgs:   []any{startLocal.Format("2006-01-02"), endLocal.Format("2006-01-02")},
			retainedSQL:  rechargeEventsSQL,
			retainedArgs: []any{startUTC, todayUTC},
		},
		{
			table:        retentionTables[RetentionTypeJoin],
			cohortSQL:    drawEventsSQL,
			cohortArgs:   []any{startUTC, endUTC},
			retainedSQL:  drawEventsSQL,
			retainedArgs: []any{startUTC, todayUTC},
		},
		{
			table:        retentionTables[RetentionTypePay],
			cohortSQL:    rechargeEventsSQL,
			cohortArgs:   []any{startUTC, endUTC},
			retainedSQL:  rechargeEventsSQL,
			retainedArgs: []any{startUTC, todayUTC},
		},
	}
	for _, s := range specs {
		if err := aggregateRetentionTable(sourceDB, reportDB, s.table, s.cohortSQL, s.cohortArgs, s.retainedSQL, s.retainedArgs, dueDate); err != nil {
			return err
		}
	}
	return nil
}

func aggregateRetentionTable(sourceDB, reportDB *gorm.DB, table, cohortSQL string, cohortArgs []any, retainedSQL string, retainedArgs []any, dueDate string) error {
	query := fmt.Sprintf(`
WITH cohorts AS (
%s
),
retained_events AS (
%s
),
totals AS (
  SELECT d AS cohort_date, COUNT(DISTINCT user_id) AS total_users
  FROM cohorts
  GROUP BY d
),
retained AS (
  SELECT c.d AS cohort_date,
         DATEDIFF(r.d, c.d) AS day_no,
         COUNT(DISTINCT c.user_id) AS retained_users
  FROM cohorts c
  JOIN retained_events r ON r.user_id = c.user_id
  WHERE DATEDIFF(r.d, c.d) IN (%s)
  GROUP BY c.d, day_no
)
SELECT DATE_FORMAT(t.cohort_date, '%%Y-%%m-%%d') AS cohort_date,
       dy.day_no,
       t.total_users,
       CASE WHEN dy.day_no = 0 THEN t.total_users ELSE COALESCE(r.retained_users, 0) END AS retained_users
FROM totals t
CROSS JOIN (
  %s
) dy
LEFT JOIN retained r ON r.cohort_date = t.cohort_date AND r.day_no = dy.day_no
WHERE DATE_ADD(t.cohort_date, INTERVAL dy.day_no DAY) <= ?`,
		cohortSQL, retainedSQL, retentionDayListSQL(), retentionDayUnionSQL())

	args := make([]any, 0, len(cohortArgs)+len(retainedArgs)+1)
	args = append(args, cohortArgs...)
	args = append(args, retainedArgs...)
	args = append(args, dueDate)

	var rows []retentionAggRow
	if err := sourceDB.Raw(query, args...).Scan(&rows).Error; err != nil {
		return fmt.Errorf("aggregate %s: %w", table, err)
	}
	if len(rows) == 0 {
		return nil
	}
	if err := reportDB.Transaction(func(tx *gorm.DB) error {
		return upsertRetentionRows(tx, table, rows)
	}); err != nil {
		return err
	}
	return nil
}

func upsertRetentionRows(tx *gorm.DB, table string, rows []retentionAggRow) error {
	const chunkSize = 500
	for start := 0; start < len(rows); start += chunkSize {
		end := start + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		var b strings.Builder
		b.WriteString("INSERT INTO gacha_report.")
		b.WriteString(table)
		b.WriteString(" (cohort_date, day_no, total_users, retained_users) VALUES ")
		args := make([]any, 0, (end-start)*4)
		for i := start; i < end; i++ {
			if i > start {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?)")
			args = append(args, rows[i].CohortDate, rows[i].DayNo, rows[i].TotalUsers, rows[i].RetainedUsers)
		}
		b.WriteString(" ON DUPLICATE KEY UPDATE total_users=VALUES(total_users), retained_users=VALUES(retained_users), updated_at=NOW(3)")
		if err := tx.Exec(b.String(), args...).Error; err != nil {
			return fmt.Errorf("upsert %s: %w", table, err)
		}
	}
	return nil
}

func retentionDayListSQL() string {
	parts := make([]string, 0, len(RetentionDays))
	for _, day := range RetentionDays {
		parts = append(parts, fmt.Sprintf("%d", day))
	}
	return strings.Join(parts, ",")
}

func retentionDayUnionSQL() string {
	var b strings.Builder
	b.WriteString("SELECT 0 AS day_no")
	for _, day := range RetentionDays {
		b.WriteString(" UNION ALL SELECT ")
		b.WriteString(fmt.Sprintf("%d", day))
	}
	return b.String()
}

// StartRetention 启动留存聚合：启动时回填最近 400 天，之后每 10 分钟刷新到期 cohort。
// 多副本通过 MySQL 命名锁串行执行，避免重复重算。
func StartRetention(sourceDB, reportDB *gorm.DB) {
	if sourceDB == nil || reportDB == nil {
		return
	}
	go func() {
		today := time.Now().In(cst)
		start := today.AddDate(0, 0, -400)
		if earliest, ok := earliestBusinessDate(sourceDB); ok && earliest.After(start) {
			start = earliest
		}
		runRetentionAggregation(sourceDB, reportDB, start, today)

		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now().In(cst)
			// 覆盖 D180 到期 cohort，以及今天/昨天 cohort 总数的增量修正。
			start := now.AddDate(0, 0, -RetentionDays[len(RetentionDays)-1])
			runRetentionAggregation(sourceDB, reportDB, start, now)
		}
	}()
}

func runRetentionAggregation(sourceDB, reportDB *gorm.DB, start, end time.Time) {
	ctx := context.Background()
	sqlDB, err := reportDB.DB()
	if err != nil {
		logx.Errorf("retention aggregate: get report sql.DB failed: %v", err)
		return
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		logx.Errorf("retention aggregate: acquire lock connection failed: %v", err)
		return
	}
	defer conn.Close()

	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", "gacha_report_retention").Scan(&got); err != nil {
		logx.Errorf("retention aggregate: acquire lock failed: %v", err)
		return
	}
	if !got.Valid || got.Int64 != 1 {
		logx.Infof("retention aggregate skipped (lock busy), start: %s, end: %s",
			start.Format("2006-01-02"), end.Format("2006-01-02"))
		return
	}
	defer func() {
		var released sql.NullInt64
		_ = conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", "gacha_report_retention").Scan(&released)
	}()

	if err := AggregateRetention(sourceDB, reportDB, start, end); err != nil {
		logx.Errorf("retention aggregate failed, start: %s, end: %s, err: %v",
			start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return
	}
	logx.Infof("retention aggregate done, start: %s, end: %s",
		start.Format("2006-01-02"), end.Format("2006-01-02"))
}
