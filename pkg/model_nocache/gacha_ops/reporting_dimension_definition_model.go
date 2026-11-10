package gacha_ops

import (
	"gorm.io/gorm"
)

var _ ReportingDimensionDefinitionModel = (*customReportingDimensionDefinitionModel)(nil)

type (
	// ReportingDimensionDefinitionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customReportingDimensionDefinitionModel.
	ReportingDimensionDefinitionModel interface {
		reportingDimensionDefinitionModel
		customReportingDimensionDefinitionLogicModel
	}

	customReportingDimensionDefinitionModel struct {
		*defaultReportingDimensionDefinitionModel
	}

	customReportingDimensionDefinitionLogicModel interface {
	}
)

// NewReportingDimensionDefinitionModel returns a model for the database table.
func NewReportingDimensionDefinitionModel(conn *gorm.DB) ReportingDimensionDefinitionModel {
	return &customReportingDimensionDefinitionModel{
		defaultReportingDimensionDefinitionModel: newReportingDimensionDefinitionModel(conn),
	}
}
