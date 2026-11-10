package gacha_ops

import (
	"gorm.io/gorm"
)

var _ ConsoleSessionsModel = (*customConsoleSessionsModel)(nil)

type (
	// ConsoleSessionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customConsoleSessionsModel.
	ConsoleSessionsModel interface {
		consoleSessionsModel
		customConsoleSessionsLogicModel
	}

	customConsoleSessionsModel struct {
		*defaultConsoleSessionsModel
	}

	customConsoleSessionsLogicModel interface {
	}
)

// NewConsoleSessionsModel returns a model for the database table.
func NewConsoleSessionsModel(conn *gorm.DB) ConsoleSessionsModel {
	return &customConsoleSessionsModel{
		defaultConsoleSessionsModel: newConsoleSessionsModel(conn),
	}
}
