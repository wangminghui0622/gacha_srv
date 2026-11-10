package gacha_play

import (
	"gorm.io/gorm"
)

var _ GashaponStateModel = (*customGashaponStateModel)(nil)

type (
	// GashaponStateModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGashaponStateModel.
	GashaponStateModel interface {
		gashaponStateModel
		customGashaponStateLogicModel
	}

	customGashaponStateModel struct {
		*defaultGashaponStateModel
	}

	customGashaponStateLogicModel interface {
	}
)

// NewGashaponStateModel returns a model for the database table.
func NewGashaponStateModel(conn *gorm.DB) GashaponStateModel {
	return &customGashaponStateModel{
		defaultGashaponStateModel: newGashaponStateModel(conn),
	}
}
