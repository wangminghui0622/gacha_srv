package gacha_play

import (
	"gorm.io/gorm"
)

var _ MemoryMatchChampionPayoutsModel = (*customMemoryMatchChampionPayoutsModel)(nil)

type (
	// MemoryMatchChampionPayoutsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchChampionPayoutsModel.
	MemoryMatchChampionPayoutsModel interface {
		memoryMatchChampionPayoutsModel
		customMemoryMatchChampionPayoutsLogicModel
	}

	customMemoryMatchChampionPayoutsModel struct {
		*defaultMemoryMatchChampionPayoutsModel
	}

	customMemoryMatchChampionPayoutsLogicModel interface {
	}
)

// NewMemoryMatchChampionPayoutsModel returns a model for the database table.
func NewMemoryMatchChampionPayoutsModel(conn *gorm.DB) MemoryMatchChampionPayoutsModel {
	return &customMemoryMatchChampionPayoutsModel{
		defaultMemoryMatchChampionPayoutsModel: newMemoryMatchChampionPayoutsModel(conn),
	}
}
