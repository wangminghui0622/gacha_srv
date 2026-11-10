package gacha_base

import (
	"gorm.io/gorm"
)

var _ RiskCaseEventsModel = (*customRiskCaseEventsModel)(nil)

type (
	// RiskCaseEventsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRiskCaseEventsModel.
	RiskCaseEventsModel interface {
		riskCaseEventsModel
		customRiskCaseEventsLogicModel
	}

	customRiskCaseEventsModel struct {
		*defaultRiskCaseEventsModel
	}

	customRiskCaseEventsLogicModel interface {
	}
)

// NewRiskCaseEventsModel returns a model for the database table.
func NewRiskCaseEventsModel(conn *gorm.DB) RiskCaseEventsModel {
	return &customRiskCaseEventsModel{
		defaultRiskCaseEventsModel: newRiskCaseEventsModel(conn),
	}
}
