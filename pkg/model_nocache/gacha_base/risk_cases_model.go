package gacha_base

import (
	"gorm.io/gorm"
)

var _ RiskCasesModel = (*customRiskCasesModel)(nil)

type (
	// RiskCasesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRiskCasesModel.
	RiskCasesModel interface {
		riskCasesModel
		customRiskCasesLogicModel
	}

	customRiskCasesModel struct {
		*defaultRiskCasesModel
	}

	customRiskCasesLogicModel interface {
	}
)

// NewRiskCasesModel returns a model for the database table.
func NewRiskCasesModel(conn *gorm.DB) RiskCasesModel {
	return &customRiskCasesModel{
		defaultRiskCasesModel: newRiskCasesModel(conn),
	}
}
