package gacha_ops

import (
	"gorm.io/gorm"
)

var _ ReportingDashboardModel = (*customReportingDashboardModel)(nil)

type (
	// ReportingDashboardModel is an interface to be customized, add more methods here,
	// and implement the added methods in customReportingDashboardModel.
	ReportingDashboardModel interface {
		reportingDashboardModel
		customReportingDashboardLogicModel
	}

	customReportingDashboardModel struct {
		*defaultReportingDashboardModel
	}

	customReportingDashboardLogicModel interface {
	}
)

// NewReportingDashboardModel returns a model for the database table.
func NewReportingDashboardModel(conn *gorm.DB) ReportingDashboardModel {
	return &customReportingDashboardModel{
		defaultReportingDashboardModel: newReportingDashboardModel(conn),
	}
}
