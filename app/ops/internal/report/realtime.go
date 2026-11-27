package report

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

const realtimeBuckets = 144 // 10 分钟一个桶，24 小时

// RealtimeSummary 实时报表顶部汇总（金额单位：分）。
type RealtimeSummary struct {
	TotalRegistrations int64 `json:"totalRegistrations"`
	TotalPayingUsers   int64 `json:"totalPayingUsers"`
	TotalPayFen        int64 `json:"totalPayFen"`
	TotalRechargeUsers int64 `json:"totalRechargeUsers"`
	TotalRechargeFen   int64 `json:"totalRechargeFen"`
	TotalRefundFen     int64 `json:"totalRefundFen"`
	ArppuFen           int64 `json:"arppuFen"`
	TodayRegistrations int64 `json:"todayRegistrations"`
	TodayPayFen        int64 `json:"todayPayFen"`
	TodayRechargeFen   int64 `json:"todayRechargeFen"`
}

// RealtimeSeries 单条曲线。kind=count|money；money 的 values 单位为分。
type RealtimeSeries struct {
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Values []int64 `json:"values"`
}

// RealtimeChart 一个实时图表。
type RealtimeChart struct {
	Title  string           `json:"title"`
	Series []RealtimeSeries `json:"series"`
}

// RealtimeResult 实时报表。
type RealtimeResult struct {
	Summary RealtimeSummary `json:"summary"`
	Labels  []string        `json:"labels"`
	Charts  []RealtimeChart `json:"charts"`
}

type bucketRow struct {
	B int   `gorm:"column:b"`
	N int64 `gorm:"column:n"`
}

// QueryRealtime 查询今天/昨天的实时数据；today 为当前时间（东八区）。
func QueryRealtime(db *gorm.DB, now time.Time) (*RealtimeResult, error) {
	loc := time.FixedZone("CST", 8*3600)
	n := now.In(loc)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	yesterday := today.AddDate(0, 0, -1)
	todayUTC := today.UTC()
	tomorrowUTC := today.AddDate(0, 0, 1).UTC()
	yesterdayUTC := yesterday.UTC()

	var summary RealtimeSummary
	if err := db.Raw("SELECT COUNT(*) FROM gacha_identity.user_users").Scan(&summary.TotalRegistrations).Error; err != nil {
		return nil, fmt.Errorf("realtime total registrations: %w", err)
	}
	var pay struct {
		Users int64
		Fen   int64
	}
	if err := db.Raw("SELECT COUNT(DISTINCT user_id) AS users, COALESCE(SUM(price), 0) AS fen FROM gacha_draw.draw_records WHERE status = 1").Scan(&pay).Error; err != nil {
		return nil, fmt.Errorf("realtime total pay: %w", err)
	}
	summary.TotalPayingUsers = pay.Users
	summary.TotalPayFen = pay.Fen
	var recharge struct {
		Users int64
		Fen   int64
	}
	if err := db.Raw("SELECT COUNT(DISTINCT user_id) AS users, COALESCE(SUM(amount_fen), 0) AS fen FROM gacha_money.recharge_orders WHERE status = 4").Scan(&recharge).Error; err != nil {
		return nil, fmt.Errorf("realtime total recharge: %w", err)
	}
	summary.TotalRechargeUsers = recharge.Users
	summary.TotalRechargeFen = recharge.Fen
	var refundRecharge, refundUser int64
	if err := db.Raw("SELECT COALESCE(SUM(refund_amount_fen), 0) FROM gacha_money.recharge_orders WHERE refund_amount_fen > 0").Scan(&refundRecharge).Error; err != nil {
		return nil, fmt.Errorf("realtime total recharge refund: %w", err)
	}
	if err := db.Raw("SELECT COALESCE(SUM(amount_fen), 0) FROM gacha_money.refund_orders WHERE status = 3").Scan(&refundUser).Error; err != nil {
		return nil, fmt.Errorf("realtime total user refund: %w", err)
	}
	summary.TotalRefundFen = refundRecharge + refundUser
	if summary.TotalPayingUsers > 0 {
		summary.ArppuFen = summary.TotalPayFen / summary.TotalPayingUsers
	}

	activeToday, err := queryActiveBuckets(db, todayUTC, tomorrowUTC)
	if err != nil {
		return nil, err
	}
	activeYesterday, err := queryActiveBuckets(db, yesterdayUTC, todayUTC)
	if err != nil {
		return nil, err
	}
	partToday, err := queryParticipationBuckets(db, todayUTC, tomorrowUTC)
	if err != nil {
		return nil, err
	}
	partYesterday, err := queryParticipationBuckets(db, yesterdayUTC, todayUTC)
	if err != nil {
		return nil, err
	}

	payToday, err := querySumBuckets(db, "gacha_draw.draw_records", "status = 1", "created_at", todayUTC, tomorrowUTC)
	if err != nil {
		return nil, err
	}
	payYesterday, err := querySumBuckets(db, "gacha_draw.draw_records", "status = 1", "created_at", yesterdayUTC, todayUTC)
	if err != nil {
		return nil, err
	}
	rechargeToday, err := querySumBuckets(db, "gacha_money.recharge_orders", "status = 4", "COALESCE(completed_at, created_at)", todayUTC, tomorrowUTC)
	if err != nil {
		return nil, err
	}
	rechargeYesterday, err := querySumBuckets(db, "gacha_money.recharge_orders", "status = 4", "COALESCE(completed_at, created_at)", yesterdayUTC, todayUTC)
	if err != nil {
		return nil, err
	}
	regsToday, err := queryCountBuckets(db, "gacha_identity.user_users", "1 = 1", "created_at", todayUTC, tomorrowUTC)
	if err != nil {
		return nil, err
	}
	regsYesterday, err := queryCountBuckets(db, "gacha_identity.user_users", "1 = 1", "created_at", yesterdayUTC, todayUTC)
	if err != nil {
		return nil, err
	}

	summary.TodayRegistrations = sumBuckets(regsToday)
	summary.TodayPayFen = sumBuckets(payToday)
	summary.TodayRechargeFen = sumBuckets(rechargeToday)

	charts := []RealtimeChart{
		{
			Title: "日活用户",
			Series: []RealtimeSeries{
				{Name: "今天日活用户", Kind: "count", Values: activeToday},
				{Name: "昨天日活用户", Kind: "count", Values: activeYesterday},
			},
		},
		{
			Title:  "参与用户",
			Series: participationSeries(db, todayUTC, tomorrowUTC, yesterdayUTC, todayUTC, partToday, partYesterday),
		},
		{
			Title: "累计付费总额",
			Series: []RealtimeSeries{
				{Name: "今天累计付费总额", Kind: "money", Values: cumulative(payToday)},
				{Name: "昨天累计付费总额", Kind: "money", Values: cumulative(payYesterday)},
			},
		},
		{
			Title: "累计充值总额",
			Series: []RealtimeSeries{
				{Name: "今天累计充值总额", Kind: "money", Values: cumulative(rechargeToday)},
				{Name: "昨天累计充值总额", Kind: "money", Values: cumulative(rechargeYesterday)},
			},
		},
		{
			Title: "新增注册",
			Series: []RealtimeSeries{
				{Name: "今天新增注册", Kind: "count", Values: regsToday},
				{Name: "昨天新增注册", Kind: "count", Values: regsYesterday},
			},
		},
		{
			Title: "累计注册",
			Series: []RealtimeSeries{
				{Name: "今天累计注册", Kind: "count", Values: cumulative(regsToday)},
				{Name: "昨天累计注册", Kind: "count", Values: cumulative(regsYesterday)},
			},
		},
	}
	return &RealtimeResult{
		Summary: summary,
		Labels:  realtimeLabels(),
		Charts:  charts,
	}, nil
}

func participationSeries(db *gorm.DB, todayStart, todayEnd, yesterdayStart, yesterdayEnd time.Time, totalToday, totalYesterday []int64) []RealtimeSeries {
	series := []RealtimeSeries{
		{Name: "今天参与用户", Kind: "count", Values: totalToday},
		{Name: "昨天参与用户", Kind: "count", Values: totalYesterday},
	}
	types := []struct {
		label  string
		table  string
		status string
	}{
		{"一番赏", "gacha_play.full_car_records", "status = 1"},
		{"吉祥赏", "gacha_play.gashapon_records", "status = 1"},
		{"串串赏", "gacha_play.combo_chain_records", "status = 1"},
		{"挖宝赏", "gacha_play.dig_box_records", "status = 1"},
		{"拼团赏", "gacha_play.group_buy_records", "status = 1"},
		{"对对碰", "gacha_play.memory_match_sessions", "status <> 3"},
	}
	for _, item := range types {
		today, err := queryDistinctUserBuckets(db, item.table, item.status, "created_at", todayStart, todayEnd)
		if err != nil {
			today = emptyBuckets()
		}
		yesterday, err := queryDistinctUserBuckets(db, item.table, item.status, "created_at", yesterdayStart, yesterdayEnd)
		if err != nil {
			yesterday = emptyBuckets()
		}
		series = append(series,
			RealtimeSeries{Name: "今天" + item.label + "参与用户", Kind: "count", Values: today},
			RealtimeSeries{Name: "昨天" + item.label + "参与用户", Kind: "count", Values: yesterday},
		)
	}
	return series
}

func queryActiveBuckets(db *gorm.DB, start, end time.Time) ([]int64, error) {
	var rows []bucketRow
	err := db.Raw(`
SELECT b, COUNT(DISTINCT user_id) AS n FROM (
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(created_at, INTERVAL 8 HOUR)) / 600) AS b, user_id
  FROM gacha_draw.draw_records WHERE status = 1 AND created_at >= ? AND created_at < ?
  UNION ALL
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(COALESCE(completed_at, created_at), INTERVAL 8 HOUR)) / 600), user_id
  FROM gacha_money.recharge_orders WHERE status = 4 AND COALESCE(completed_at, created_at) >= ? AND COALESCE(completed_at, created_at) < ?
  UNION ALL
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(created_at, INTERVAL 8 HOUR)) / 600), user_id
  FROM gacha_money.payment_orders WHERE status = 3 AND created_at >= ? AND created_at < ?
  UNION ALL
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(COALESCE(completed_at, created_at), INTERVAL 8 HOUR)) / 600), user_id
  FROM gacha_money.refund_orders WHERE status = 3 AND COALESCE(completed_at, created_at) >= ? AND COALESCE(completed_at, created_at) < ?
) x GROUP BY b`,
		start, end, start, end, start, end, start, end,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("realtime active buckets: %w", err)
	}
	return fillBuckets(rows), nil
}

func queryParticipationBuckets(db *gorm.DB, start, end time.Time) ([]int64, error) {
	var rows []bucketRow
	err := db.Raw(`
SELECT b, COUNT(DISTINCT user_id) AS n FROM (
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(created_at, INTERVAL 8 HOUR)) / 600) AS b, user_id
  FROM gacha_draw.draw_records WHERE status = 1 AND created_at >= ? AND created_at < ?
  UNION ALL
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(created_at, INTERVAL 8 HOUR)) / 600), user_id
  FROM gacha_play.full_car_records WHERE status = 1 AND created_at >= ? AND created_at < ?
  UNION ALL
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(created_at, INTERVAL 8 HOUR)) / 600), user_id
  FROM gacha_play.gashapon_records WHERE status = 1 AND created_at >= ? AND created_at < ?
  UNION ALL
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(created_at, INTERVAL 8 HOUR)) / 600), user_id
  FROM gacha_play.combo_chain_records WHERE status = 1 AND created_at >= ? AND created_at < ?
  UNION ALL
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(created_at, INTERVAL 8 HOUR)) / 600), user_id
  FROM gacha_play.dig_box_records WHERE status = 1 AND created_at >= ? AND created_at < ?
  UNION ALL
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(created_at, INTERVAL 8 HOUR)) / 600), user_id
  FROM gacha_play.group_buy_records WHERE status = 1 AND created_at >= ? AND created_at < ?
  UNION ALL
  SELECT FLOOR(TIME_TO_SEC(DATE_ADD(created_at, INTERVAL 8 HOUR)) / 600), user_id
  FROM gacha_play.memory_match_sessions WHERE status <> 3 AND created_at >= ? AND created_at < ?
) x GROUP BY b`,
		start, end, start, end, start, end, start, end, start, end, start, end, start, end,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("realtime participation buckets: %w", err)
	}
	return fillBuckets(rows), nil
}

func queryDistinctUserBuckets(db *gorm.DB, table, where, eventExpr string, start, end time.Time) ([]int64, error) {
	var rows []bucketRow
	query := fmt.Sprintf(`
SELECT FLOOR(TIME_TO_SEC(DATE_ADD(%s, INTERVAL 8 HOUR)) / 600) AS b, COUNT(DISTINCT user_id) AS n
FROM %s WHERE %s AND %s >= ? AND %s < ? GROUP BY b`, eventExpr, table, where, eventExpr, eventExpr)
	if err := db.Raw(query, start, end).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("realtime buckets %s: %w", table, err)
	}
	return fillBuckets(rows), nil
}

func querySumBuckets(db *gorm.DB, table, where, eventExpr string, start, end time.Time) ([]int64, error) {
	var rows []bucketRow
	query := fmt.Sprintf(`
SELECT FLOOR(TIME_TO_SEC(DATE_ADD(%s, INTERVAL 8 HOUR)) / 600) AS b, COALESCE(SUM(price), 0) AS n
FROM %s WHERE %s AND %s >= ? AND %s < ? GROUP BY b`, eventExpr, table, where, eventExpr, eventExpr)
	if table == "gacha_money.recharge_orders" {
		query = fmt.Sprintf(`
SELECT FLOOR(TIME_TO_SEC(DATE_ADD(%s, INTERVAL 8 HOUR)) / 600) AS b, COALESCE(SUM(amount_fen), 0) AS n
FROM %s WHERE %s AND %s >= ? AND %s < ? GROUP BY b`, eventExpr, table, where, eventExpr, eventExpr)
	}
	if err := db.Raw(query, start, end).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("realtime sum buckets %s: %w", table, err)
	}
	return fillBuckets(rows), nil
}

func queryCountBuckets(db *gorm.DB, table, where, eventExpr string, start, end time.Time) ([]int64, error) {
	var rows []bucketRow
	query := fmt.Sprintf(`
SELECT FLOOR(TIME_TO_SEC(DATE_ADD(%s, INTERVAL 8 HOUR)) / 600) AS b, COUNT(*) AS n
FROM %s WHERE %s AND %s >= ? AND %s < ? GROUP BY b`, eventExpr, table, where, eventExpr, eventExpr)
	if err := db.Raw(query, start, end).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("realtime count buckets %s: %w", table, err)
	}
	return fillBuckets(rows), nil
}

func emptyBuckets() []int64 {
	return make([]int64, realtimeBuckets)
}

func fillBuckets(rows []bucketRow) []int64 {
	out := make([]int64, realtimeBuckets)
	for _, row := range rows {
		if row.B >= 0 && row.B < realtimeBuckets {
			out[row.B] = row.N
		}
	}
	return out
}

func cumulative(values []int64) []int64 {
	out := make([]int64, len(values))
	var total int64
	for i, value := range values {
		total += value
		out[i] = total
	}
	return out
}

func sumBuckets(values []int64) int64 {
	var total int64
	for _, value := range values {
		total += value
	}
	return total
}

func realtimeLabels() []string {
	labels := make([]string, realtimeBuckets)
	for i := 0; i < realtimeBuckets; i++ {
		labels[i] = fmt.Sprintf("%02d:%02d", i/6, (i%6)*10)
	}
	return labels
}
