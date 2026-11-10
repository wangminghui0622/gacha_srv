package gacha_play

import (
	"gorm.io/gorm"
)

var _ MemoryMatchAppearanceEntitlementsModel = (*customMemoryMatchAppearanceEntitlementsModel)(nil)

type (
	// MemoryMatchAppearanceEntitlementsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchAppearanceEntitlementsModel.
	MemoryMatchAppearanceEntitlementsModel interface {
		memoryMatchAppearanceEntitlementsModel
		customMemoryMatchAppearanceEntitlementsLogicModel
	}

	customMemoryMatchAppearanceEntitlementsModel struct {
		*defaultMemoryMatchAppearanceEntitlementsModel
	}

	customMemoryMatchAppearanceEntitlementsLogicModel interface {
	}
)

// NewMemoryMatchAppearanceEntitlementsModel returns a model for the database table.
func NewMemoryMatchAppearanceEntitlementsModel(conn *gorm.DB) MemoryMatchAppearanceEntitlementsModel {
	return &customMemoryMatchAppearanceEntitlementsModel{
		defaultMemoryMatchAppearanceEntitlementsModel: newMemoryMatchAppearanceEntitlementsModel(conn),
	}
}
