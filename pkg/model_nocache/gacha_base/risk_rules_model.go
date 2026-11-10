package gacha_base

import (
	"gorm.io/gorm"
)

var _ RiskRulesModel = (*customRiskRulesModel)(nil)

type (
	// RiskRulesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRiskRulesModel.
	RiskRulesModel interface {
		riskRulesModel
		customRiskRulesLogicModel
	}

	customRiskRulesModel struct {
		*defaultRiskRulesModel
	}

	customRiskRulesLogicModel interface {
	}
)

// NewRiskRulesModel returns a model for the database table.
func NewRiskRulesModel(conn *gorm.DB) RiskRulesModel {
	return &customRiskRulesModel{
		defaultRiskRulesModel: newRiskRulesModel(conn),
	}
}
