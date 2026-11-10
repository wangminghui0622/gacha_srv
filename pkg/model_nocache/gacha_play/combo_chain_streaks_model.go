package gacha_play

import (
	"gorm.io/gorm"
)

var _ ComboChainStreaksModel = (*customComboChainStreaksModel)(nil)

type (
	// ComboChainStreaksModel is an interface to be customized, add more methods here,
	// and implement the added methods in customComboChainStreaksModel.
	ComboChainStreaksModel interface {
		comboChainStreaksModel
		customComboChainStreaksLogicModel
	}

	customComboChainStreaksModel struct {
		*defaultComboChainStreaksModel
	}

	customComboChainStreaksLogicModel interface {
	}
)

// NewComboChainStreaksModel returns a model for the database table.
func NewComboChainStreaksModel(conn *gorm.DB) ComboChainStreaksModel {
	return &customComboChainStreaksModel{
		defaultComboChainStreaksModel: newComboChainStreaksModel(conn),
	}
}
