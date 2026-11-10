package gacha_play

import (
	"gorm.io/gorm"
)

var _ ComboChainStateModel = (*customComboChainStateModel)(nil)

type (
	// ComboChainStateModel is an interface to be customized, add more methods here,
	// and implement the added methods in customComboChainStateModel.
	ComboChainStateModel interface {
		comboChainStateModel
		customComboChainStateLogicModel
	}

	customComboChainStateModel struct {
		*defaultComboChainStateModel
	}

	customComboChainStateLogicModel interface {
	}
)

// NewComboChainStateModel returns a model for the database table.
func NewComboChainStateModel(conn *gorm.DB) ComboChainStateModel {
	return &customComboChainStateModel{
		defaultComboChainStateModel: newComboChainStateModel(conn),
	}
}
