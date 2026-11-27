package report

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// MarketRow 市场报表每日明细（金额单位：分）。
type MarketRow struct {
	Date            string `json:"date"`
	Participants    int64  `json:"participants"`
	NewOrders       int64  `json:"newOrders"`
	AmountTotalFen  int64  `json:"amountTotalFen"`
	AmountNormalFen int64  `json:"amountNormalFen"`
	AmountSampleFen int64  `json:"amountSampleFen"`
}

// MarketResult 市场报表。
type MarketResult struct {
	Rows []MarketRow `json:"rows"`
}

// QueryMarket 从 gacha_report.market_daily 查询 [start, end] 的市场报表。
// 表里没有数据的日期补 0，保证前端按天展示。
func QueryMarket(reportDB *gorm.DB, start, end time.Time) (*MarketResult, error) {
	startLocal := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, cst)
	endLocal := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, cst)
	if endLocal.Before(startLocal) {
		return nil, fmt.Errorf("end date before start date")
	}

	var dbRows []MarketRow
	if err := reportDB.Raw(`
SELECT DATE_FORMAT(report_date, '%Y-%m-%d') AS date,
       participants, new_orders, amount_total_fen, amount_normal_fen, amount_sample_fen
FROM gacha_report.market_daily
WHERE report_date >= ? AND report_date <= ?
ORDER BY report_date`,
		startLocal.Format("2006-01-02"), endLocal.Format("2006-01-02"),
	).Scan(&dbRows).Error; err != nil {
		return nil, fmt.Errorf("query market rows: %w", err)
	}
	byDate := make(map[string]MarketRow, len(dbRows))
	for _, row := range dbRows {
		byDate[row.Date] = row
	}
	rows := make([]MarketRow, 0, daysBetween(startLocal, endLocal)+1)
	for d := startLocal; !d.After(endLocal); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if row, ok := byDate[key]; ok {
			rows = append(rows, row)
			continue
		}
		rows = append(rows, MarketRow{Date: key})
	}
	return &MarketResult{Rows: rows}, nil
}
