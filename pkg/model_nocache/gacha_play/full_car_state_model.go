package gacha_play

import (
	"gorm.io/gorm"
)

var _ FullCarStateModel = (*customFullCarStateModel)(nil)

type (
	// FullCarStateModel is an interface to be customized, add more methods here,
	// and implement the added methods in customFullCarStateModel.
	FullCarStateModel interface {
		fullCarStateModel
		customFullCarStateLogicModel
	}

	customFullCarStateModel struct {
		*defaultFullCarStateModel
	}

	customFullCarStateLogicModel interface {
	}
)

// NewFullCarStateModel returns a model for the database table.
func NewFullCarStateModel(conn *gorm.DB) FullCarStateModel {
	return &customFullCarStateModel{
		defaultFullCarStateModel: newFullCarStateModel(conn),
	}
}
