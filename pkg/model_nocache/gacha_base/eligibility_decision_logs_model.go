package gacha_base

import (
	"gorm.io/gorm"
)

var _ EligibilityDecisionLogsModel = (*customEligibilityDecisionLogsModel)(nil)

type (
	// EligibilityDecisionLogsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customEligibilityDecisionLogsModel.
	EligibilityDecisionLogsModel interface {
		eligibilityDecisionLogsModel
		customEligibilityDecisionLogsLogicModel
	}

	customEligibilityDecisionLogsModel struct {
		*defaultEligibilityDecisionLogsModel
	}

	customEligibilityDecisionLogsLogicModel interface {
	}
)

// NewEligibilityDecisionLogsModel returns a model for the database table.
func NewEligibilityDecisionLogsModel(conn *gorm.DB) EligibilityDecisionLogsModel {
	return &customEligibilityDecisionLogsModel{
		defaultEligibilityDecisionLogsModel: newEligibilityDecisionLogsModel(conn),
	}
}
