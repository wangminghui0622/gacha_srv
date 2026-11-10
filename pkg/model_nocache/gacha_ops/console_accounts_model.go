package gacha_ops

import (
	"gorm.io/gorm"
)

var _ ConsoleAccountsModel = (*customConsoleAccountsModel)(nil)

type (
	// ConsoleAccountsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customConsoleAccountsModel.
	ConsoleAccountsModel interface {
		consoleAccountsModel
		customConsoleAccountsLogicModel
	}

	customConsoleAccountsModel struct {
		*defaultConsoleAccountsModel
	}

	customConsoleAccountsLogicModel interface {
	}
)

// NewConsoleAccountsModel returns a model for the database table.
func NewConsoleAccountsModel(conn *gorm.DB) ConsoleAccountsModel {
	return &customConsoleAccountsModel{
		defaultConsoleAccountsModel: newConsoleAccountsModel(conn),
	}
}
