package gacha_play

import (
	"gorm.io/gorm"
)

var _ GashaponRecordsModel = (*customGashaponRecordsModel)(nil)

type (
	// GashaponRecordsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGashaponRecordsModel.
	GashaponRecordsModel interface {
		gashaponRecordsModel
		customGashaponRecordsLogicModel
	}

	customGashaponRecordsModel struct {
		*defaultGashaponRecordsModel
	}

	customGashaponRecordsLogicModel interface {
	}
)

// NewGashaponRecordsModel returns a model for the database table.
func NewGashaponRecordsModel(conn *gorm.DB) GashaponRecordsModel {
	return &customGashaponRecordsModel{
		defaultGashaponRecordsModel: newGashaponRecordsModel(conn),
	}
}
