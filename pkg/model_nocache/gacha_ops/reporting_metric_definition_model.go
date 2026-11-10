package gacha_ops

import (
	"gorm.io/gorm"
)

var _ ReportingMetricDefinitionModel = (*customReportingMetricDefinitionModel)(nil)

type (
	// ReportingMetricDefinitionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customReportingMetricDefinitionModel.
	ReportingMetricDefinitionModel interface {
		reportingMetricDefinitionModel
		customReportingMetricDefinitionLogicModel
	}

	customReportingMetricDefinitionModel struct {
		*defaultReportingMetricDefinitionModel
	}

	customReportingMetricDefinitionLogicModel interface {
	}
)

// NewReportingMetricDefinitionModel returns a model for the database table.
func NewReportingMetricDefinitionModel(conn *gorm.DB) ReportingMetricDefinitionModel {
	return &customReportingMetricDefinitionModel{
		defaultReportingMetricDefinitionModel: newReportingMetricDefinitionModel(conn),
	}
}
