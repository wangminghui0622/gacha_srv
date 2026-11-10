package gacha_ops

import (
	"gorm.io/gorm"
)

var _ ReportingDashboardWidgetModel = (*customReportingDashboardWidgetModel)(nil)

type (
	// ReportingDashboardWidgetModel is an interface to be customized, add more methods here,
	// and implement the added methods in customReportingDashboardWidgetModel.
	ReportingDashboardWidgetModel interface {
		reportingDashboardWidgetModel
		customReportingDashboardWidgetLogicModel
	}

	customReportingDashboardWidgetModel struct {
		*defaultReportingDashboardWidgetModel
	}

	customReportingDashboardWidgetLogicModel interface {
	}
)

// NewReportingDashboardWidgetModel returns a model for the database table.
func NewReportingDashboardWidgetModel(conn *gorm.DB) ReportingDashboardWidgetModel {
	return &customReportingDashboardWidgetModel{
		defaultReportingDashboardWidgetModel: newReportingDashboardWidgetModel(conn),
	}
}
