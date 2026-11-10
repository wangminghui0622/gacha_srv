package gacha_play

import (
	"gorm.io/gorm"
)

var _ MemoryMatchChampionEventsModel = (*customMemoryMatchChampionEventsModel)(nil)

type (
	// MemoryMatchChampionEventsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchChampionEventsModel.
	MemoryMatchChampionEventsModel interface {
		memoryMatchChampionEventsModel
		customMemoryMatchChampionEventsLogicModel
	}

	customMemoryMatchChampionEventsModel struct {
		*defaultMemoryMatchChampionEventsModel
	}

	customMemoryMatchChampionEventsLogicModel interface {
	}
)

// NewMemoryMatchChampionEventsModel returns a model for the database table.
func NewMemoryMatchChampionEventsModel(conn *gorm.DB) MemoryMatchChampionEventsModel {
	return &customMemoryMatchChampionEventsModel{
		defaultMemoryMatchChampionEventsModel: newMemoryMatchChampionEventsModel(conn),
	}
}
