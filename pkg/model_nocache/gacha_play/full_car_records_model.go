package gacha_play

import (
	"gorm.io/gorm"
)

var _ FullCarRecordsModel = (*customFullCarRecordsModel)(nil)

type (
	// FullCarRecordsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customFullCarRecordsModel.
	FullCarRecordsModel interface {
		fullCarRecordsModel
		customFullCarRecordsLogicModel
	}

	customFullCarRecordsModel struct {
		*defaultFullCarRecordsModel
	}

	customFullCarRecordsLogicModel interface {
	}
)

// NewFullCarRecordsModel returns a model for the database table.
func NewFullCarRecordsModel(conn *gorm.DB) FullCarRecordsModel {
	return &customFullCarRecordsModel{
		defaultFullCarRecordsModel: newFullCarRecordsModel(conn),
	}
}
