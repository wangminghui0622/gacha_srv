package gacha_play

import (
	"gorm.io/gorm"
)

var _ MemoryMatchSessionsModel = (*customMemoryMatchSessionsModel)(nil)

type (
	// MemoryMatchSessionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchSessionsModel.
	MemoryMatchSessionsModel interface {
		memoryMatchSessionsModel
		customMemoryMatchSessionsLogicModel
	}

	customMemoryMatchSessionsModel struct {
		*defaultMemoryMatchSessionsModel
	}

	customMemoryMatchSessionsLogicModel interface {
	}
)

// NewMemoryMatchSessionsModel returns a model for the database table.
func NewMemoryMatchSessionsModel(conn *gorm.DB) MemoryMatchSessionsModel {
	return &customMemoryMatchSessionsModel{
		defaultMemoryMatchSessionsModel: newMemoryMatchSessionsModel(conn),
	}
}
