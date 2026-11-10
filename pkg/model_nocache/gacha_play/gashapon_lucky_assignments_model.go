package gacha_play

import (
	"gorm.io/gorm"
)

var _ GashaponLuckyAssignmentsModel = (*customGashaponLuckyAssignmentsModel)(nil)

type (
	// GashaponLuckyAssignmentsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGashaponLuckyAssignmentsModel.
	GashaponLuckyAssignmentsModel interface {
		gashaponLuckyAssignmentsModel
		customGashaponLuckyAssignmentsLogicModel
	}

	customGashaponLuckyAssignmentsModel struct {
		*defaultGashaponLuckyAssignmentsModel
	}

	customGashaponLuckyAssignmentsLogicModel interface {
	}
)

// NewGashaponLuckyAssignmentsModel returns a model for the database table.
func NewGashaponLuckyAssignmentsModel(conn *gorm.DB) GashaponLuckyAssignmentsModel {
	return &customGashaponLuckyAssignmentsModel{
		defaultGashaponLuckyAssignmentsModel: newGashaponLuckyAssignmentsModel(conn),
	}
}
