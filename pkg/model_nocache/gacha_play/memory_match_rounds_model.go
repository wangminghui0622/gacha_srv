package gacha_play

import (
	"gorm.io/gorm"
)

var _ MemoryMatchRoundsModel = (*customMemoryMatchRoundsModel)(nil)

type (
	// MemoryMatchRoundsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchRoundsModel.
	MemoryMatchRoundsModel interface {
		memoryMatchRoundsModel
		customMemoryMatchRoundsLogicModel
	}

	customMemoryMatchRoundsModel struct {
		*defaultMemoryMatchRoundsModel
	}

	customMemoryMatchRoundsLogicModel interface {
	}
)

// NewMemoryMatchRoundsModel returns a model for the database table.
func NewMemoryMatchRoundsModel(conn *gorm.DB) MemoryMatchRoundsModel {
	return &customMemoryMatchRoundsModel{
		defaultMemoryMatchRoundsModel: newMemoryMatchRoundsModel(conn),
	}
}
