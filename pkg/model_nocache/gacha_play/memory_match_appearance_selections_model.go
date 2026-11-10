package gacha_play

import (
	"gorm.io/gorm"
)

var _ MemoryMatchAppearanceSelectionsModel = (*customMemoryMatchAppearanceSelectionsModel)(nil)

type (
	// MemoryMatchAppearanceSelectionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchAppearanceSelectionsModel.
	MemoryMatchAppearanceSelectionsModel interface {
		memoryMatchAppearanceSelectionsModel
		customMemoryMatchAppearanceSelectionsLogicModel
	}

	customMemoryMatchAppearanceSelectionsModel struct {
		*defaultMemoryMatchAppearanceSelectionsModel
	}

	customMemoryMatchAppearanceSelectionsLogicModel interface {
	}
)

// NewMemoryMatchAppearanceSelectionsModel returns a model for the database table.
func NewMemoryMatchAppearanceSelectionsModel(conn *gorm.DB) MemoryMatchAppearanceSelectionsModel {
	return &customMemoryMatchAppearanceSelectionsModel{
		defaultMemoryMatchAppearanceSelectionsModel: newMemoryMatchAppearanceSelectionsModel(conn),
	}
}
