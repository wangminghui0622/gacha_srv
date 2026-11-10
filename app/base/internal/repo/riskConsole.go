// Package repo M14d 控制台数据访问（base 域）：风控案件状态机（claim/resolve 行锁 + 迁移事件）。
package repo

import (
	"time"

	"gorm.io/gorm"
)

// 案件/事件状态常量（对齐 design D4；无类型常量便于 model uint64 与行 struct int64 共用）
const (
	RiskCaseStatusOpen     = 1
	RiskCaseStatusInReview = 2
	RiskCaseStatusResolved = 3
	RiskCaseStatusDismissed = 4

	RiskEventTypeOpened    = 1
	RiskEventTypeClaimed   = 2
	RiskEventTypeResolved  = 3
	RiskEventTypeDismissed = 4
)

// RiskCaseRow risk_cases 表行（仅状态机所需列；行锁直用 GORM）
type RiskCaseRow struct {
	ID              int64 `gorm:"column:id;primaryKey;autoIncrement"`
	CaseNo          string
	DecisionID      string `gorm:"column:decision_id"`
	RuleNo          string
	Scene           string
	UserID          int64 `gorm:"column:user_id"`
	BoxID           int64 `gorm:"column:box_id"`
	AmountFen       int64
	RiskScore       int64
	Action          int64
	Status          int64
	Claimant        string
	AssignedActorID int64 `gorm:"column:assigned_actor_id"`
	ResolutionCode  string
	Resolution      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (RiskCaseRow) TableName() string { return "risk_cases" }

// RiskCaseEventRow risk_case_events 表行（event_id=operation_id uk，幂等重放凭据）
type RiskCaseEventRow struct {
	EventID        string `gorm:"column:event_id;primaryKey"`
	CaseID         string `gorm:"column:case_id"`
	EventType      int64  `gorm:"column:event_type"`
	FromStatus     int64  `gorm:"column:from_status"`
	ToStatus       int64  `gorm:"column:to_status"`
	ActorID        int64  `gorm:"column:actor_id"`
	ResolutionCode string
	OccurredAt     time.Time
}

func (RiskCaseEventRow) TableName() string { return "risk_case_events" }

// LockRiskCaseByCaseNo 行锁案件（FOR UPDATE；Claim/Resolve 串行化）；未找到返回 gorm.ErrRecordNotFound。
func LockRiskCaseByCaseNo(tx *gorm.DB, caseNo string) (*RiskCaseRow, error) {
	var row RiskCaseRow
	res := tx.Raw(
		"SELECT id, case_no, decision_id, rule_no, scene, user_id, box_id, amount_fen, "+
			"risk_score, action, status, claimant, assigned_actor_id, resolution_code, resolution, "+
			"created_at, updated_at FROM risk_cases WHERE case_no = ? FOR UPDATE", caseNo).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// GetRiskCaseEvent 按 event_id 查迁移事件（同 operation_id 重放判断）
func GetRiskCaseEvent(tx *gorm.DB, eventID string) (*RiskCaseEventRow, error) {
	var row RiskCaseEventRow
	err := tx.Where("event_id = ?", eventID).First(&row).Error
	return &row, err
}

// InsertRiskCaseEvent 插迁移事件（uk event_id；并发撞键由调用方按冲突处理）
func InsertRiskCaseEvent(tx *gorm.DB, row *RiskCaseEventRow) error {
	return tx.Create(row).Error
}

// UpdateRiskCaseStatus 更新案件状态/认领人/处置码
func UpdateRiskCaseStatus(tx *gorm.DB, row *RiskCaseRow) error {
	return tx.Model(&RiskCaseRow{}).Where("id = ?", row.ID).
		Updates(map[string]any{
			"status":            row.Status,
			"assigned_actor_id": row.AssignedActorID,
			"resolution_code":   row.ResolutionCode,
		}).Error
}

// ListRiskCasePage 案件游标分页：(created_at, id) 双键倒序，取 limit+1 判断有无下一页
func ListRiskCasePage(tx *gorm.DB, assignedActorID int64, status int64,
	beforeCreatedAt time.Time, beforeCaseID string, limit int) ([]RiskCaseRow, error) {
	q := tx
	if assignedActorID > 0 {
		q = q.Where("assigned_actor_id = ?", assignedActorID)
	}
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	if !beforeCreatedAt.IsZero() {
		q = q.Where("created_at < ? OR (created_at = ? AND case_no < ?)",
			beforeCreatedAt, beforeCreatedAt, beforeCaseID)
	}
	var rows []RiskCaseRow
	err := q.Order("created_at DESC, case_no DESC").Limit(limit + 1).Find(&rows).Error
	return rows, err
}
