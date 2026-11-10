package gacha_money

import (
	"gorm.io/gorm"
)

var _ UserBalanceLogsModel = (*customUserBalanceLogsModel)(nil)

type (
	// UserBalanceLogsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserBalanceLogsModel.
	UserBalanceLogsModel interface {
		userBalanceLogsModel
		customUserBalanceLogsLogicModel
	}

	customUserBalanceLogsModel struct {
		*defaultUserBalanceLogsModel
	}

	customUserBalanceLogsLogicModel interface {
	}
)

// NewUserBalanceLogsModel returns a model for the database table.
func NewUserBalanceLogsModel(conn *gorm.DB) UserBalanceLogsModel {
	return &customUserBalanceLogsModel{
		defaultUserBalanceLogsModel: newUserBalanceLogsModel(conn),
	}
}
