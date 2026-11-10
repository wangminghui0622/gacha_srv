package gacha_play

import (
	"gorm.io/gorm"
)

var _ DigBoxSessionsModel = (*customDigBoxSessionsModel)(nil)

type (
	// DigBoxSessionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDigBoxSessionsModel.
	DigBoxSessionsModel interface {
		digBoxSessionsModel
		customDigBoxSessionsLogicModel
	}

	customDigBoxSessionsModel struct {
		*defaultDigBoxSessionsModel
	}

	customDigBoxSessionsLogicModel interface {
	}
)

// NewDigBoxSessionsModel returns a model for the database table.
func NewDigBoxSessionsModel(conn *gorm.DB) DigBoxSessionsModel {
	return &customDigBoxSessionsModel{
		defaultDigBoxSessionsModel: newDigBoxSessionsModel(conn),
	}
}
