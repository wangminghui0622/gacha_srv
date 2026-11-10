package gacha_money

import (
	"gorm.io/gorm"
)

var _ UserBalancesModel = (*customUserBalancesModel)(nil)

type (
	// UserBalancesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserBalancesModel.
	UserBalancesModel interface {
		userBalancesModel
		customUserBalancesLogicModel
	}

	customUserBalancesModel struct {
		*defaultUserBalancesModel
	}

	customUserBalancesLogicModel interface {
	}
)

// NewUserBalancesModel returns a model for the database table.
func NewUserBalancesModel(conn *gorm.DB) UserBalancesModel {
	return &customUserBalancesModel{
		defaultUserBalancesModel: newUserBalancesModel(conn),
	}
}
