package report

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// OrderRow 订单报表每日每用户汇总（金额单位：分）。
type OrderRow struct {
	Date     string `json:"date"`
	UserID   int64  `json:"userId"`
	Nickname string `json:"nickname"`
	CostFen  int64  `json:"costFen"`
}

// OrderResult 订单报表结果。
type OrderResult struct {
	Rows []OrderRow `json:"rows"`
}

// QueryOrder 按日期+用户聚合抽赏订单：成本价 = 当日该用户实付抽赏金额合计。
// 日期按东八区自然日展示；draw_records.created_at 以 UTC 存储。
func QueryOrder(db *gorm.DB, start, end time.Time, userID, nickname string, phoneUserID int64) (*OrderResult, error) {
	startLocal := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, cst)
	endLocal := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, cst)
	if endLocal.Before(startLocal) {
		return nil, fmt.Errorf("end date before start date")
	}
	startUTC := startLocal.UTC()
	endUTC := endLocal.AddDate(0, 0, 1).UTC()

	where := []string{
		"r.status = 1",
		"r.created_at >= ?",
		"r.created_at < ?",
	}
	args := []any{startUTC, endUTC}

	if raw := strings.TrimSpace(userID); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			if id > 0 {
				where = append(where, "r.user_id = ?")
				args = append(args, id)
			}
		} else {
			where = append(where, "u.public_id = ?")
			args = append(args, raw)
		}
	}
	if raw := strings.TrimSpace(nickname); raw != "" {
		where = append(where, "u.nickname LIKE ?")
		args = append(args, "%"+raw+"%")
	}
	if phoneUserID > 0 {
		where = append(where, "r.user_id = ?")
		args = append(args, phoneUserID)
	}

	query := `
SELECT DATE_FORMAT(DATE_ADD(r.created_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS date,
       r.user_id AS user_id,
       COALESCE(MAX(u.nickname), '') AS nickname,
       COALESCE(SUM(r.price), 0) AS cost_fen
FROM gacha_draw.draw_records r
LEFT JOIN gacha_identity.user_users u ON u.id = r.user_id
WHERE ` + strings.Join(where, " AND ") + `
GROUP BY date, r.user_id
ORDER BY date DESC, r.user_id DESC`

	var rows []OrderRow
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query order rows: %w", err)
	}
	return &OrderResult{Rows: rows}, nil
}
