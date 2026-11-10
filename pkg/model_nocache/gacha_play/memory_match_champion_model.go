package gacha_play

import (
	"gorm.io/gorm"
)

var _ MemoryMatchChampionModel = (*customMemoryMatchChampionModel)(nil)

type (
	// MemoryMatchChampionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchChampionModel.
	MemoryMatchChampionModel interface {
		memoryMatchChampionModel
		customMemoryMatchChampionLogicModel
	}

	customMemoryMatchChampionModel struct {
		*defaultMemoryMatchChampionModel
	}

	customMemoryMatchChampionLogicModel interface {
	}
)

// NewMemoryMatchChampionModel returns a model for the database table.
func NewMemoryMatchChampionModel(conn *gorm.DB) MemoryMatchChampionModel {
	return &customMemoryMatchChampionModel{
		defaultMemoryMatchChampionModel: newMemoryMatchChampionModel(conn),
	}
}
