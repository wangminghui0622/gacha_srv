package gacha_play

import (
	"gorm.io/gorm"
)

var _ ComboChainRecordsModel = (*customComboChainRecordsModel)(nil)

type (
	// ComboChainRecordsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customComboChainRecordsModel.
	ComboChainRecordsModel interface {
		comboChainRecordsModel
		customComboChainRecordsLogicModel
	}

	customComboChainRecordsModel struct {
		*defaultComboChainRecordsModel
	}

	customComboChainRecordsLogicModel interface {
	}
)

// NewComboChainRecordsModel returns a model for the database table.
func NewComboChainRecordsModel(conn *gorm.DB) ComboChainRecordsModel {
	return &customComboChainRecordsModel{
		defaultComboChainRecordsModel: newComboChainRecordsModel(conn),
	}
}
