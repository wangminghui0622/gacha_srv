package gacha_ops

import (
	"context"

	"gorm.io/gorm"
)

var _ ReportingMetricInboxModel = (*customReportingMetricInboxModel)(nil)

type (
	// ReportingMetricInboxModel is an interface to be customized, add more methods here,
	// and implement the added methods in customReportingMetricInboxModel.
	ReportingMetricInboxModel interface {
		reportingMetricInboxModel
		customReportingMetricInboxLogicModel
	}

	customReportingMetricInboxModel struct {
		*defaultReportingMetricInboxModel
	}

	customReportingMetricInboxLogicModel interface {
		// InsertIgnore event_id 幂等去重（server_v2 InsertMetricInbox INSERT IGNORE 同款）：
		// 返回 RowsAffected，0 ⇒ 事件已处理过（applied=false）。
		InsertIgnore(ctx context.Context, tx *gorm.DB, data *ReportingMetricInbox) (int64, error)
	}
)

// NewReportingMetricInboxModel returns a model for the database table.
func NewReportingMetricInboxModel(conn *gorm.DB) ReportingMetricInboxModel {
	return &customReportingMetricInboxModel{
		defaultReportingMetricInboxModel: newReportingMetricInboxModel(conn),
	}
}

// InsertIgnore 幂等插入（INSERT IGNORE；event_id 主键冲突时 0 行受影响）。
func (m *customReportingMetricInboxModel) InsertIgnore(ctx context.Context, tx *gorm.DB, data *ReportingMetricInbox) (int64, error) {
	db := m.conn
	if tx != nil {
		db = tx
	}
	res := db.WithContext(ctx).Exec(
		"INSERT IGNORE INTO `reporting_metric_inbox`(`event_id`,`tenant_id`,`event_type`,`source_revision`,`received_at`) VALUES (?,?,?,?,?)",
		data.EventId, data.TenantId, data.EventType, data.SourceRevision, data.ReceivedAt)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
