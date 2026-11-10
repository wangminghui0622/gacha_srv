package gacha_play

import (
	"gorm.io/gorm"
)

var _ DigBoxRecordsModel = (*customDigBoxRecordsModel)(nil)

type (
	// DigBoxRecordsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDigBoxRecordsModel.
	DigBoxRecordsModel interface {
		digBoxRecordsModel
		customDigBoxRecordsLogicModel
	}

	customDigBoxRecordsModel struct {
		*defaultDigBoxRecordsModel
	}

	customDigBoxRecordsLogicModel interface {
	}
)

// NewDigBoxRecordsModel returns a model for the database table.
func NewDigBoxRecordsModel(conn *gorm.DB) DigBoxRecordsModel {
	return &customDigBoxRecordsModel{
		defaultDigBoxRecordsModel: newDigBoxRecordsModel(conn),
	}
}
