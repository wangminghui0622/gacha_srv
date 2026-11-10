package gacha_play

import (
	"gorm.io/gorm"
)

var _ MemoryMatchSettlementsModel = (*customMemoryMatchSettlementsModel)(nil)

type (
	// MemoryMatchSettlementsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchSettlementsModel.
	MemoryMatchSettlementsModel interface {
		memoryMatchSettlementsModel
		customMemoryMatchSettlementsLogicModel
	}

	customMemoryMatchSettlementsModel struct {
		*defaultMemoryMatchSettlementsModel
	}

	customMemoryMatchSettlementsLogicModel interface {
	}
)

// NewMemoryMatchSettlementsModel returns a model for the database table.
func NewMemoryMatchSettlementsModel(conn *gorm.DB) MemoryMatchSettlementsModel {
	return &customMemoryMatchSettlementsModel{
		defaultMemoryMatchSettlementsModel: newMemoryMatchSettlementsModel(conn),
	}
}
