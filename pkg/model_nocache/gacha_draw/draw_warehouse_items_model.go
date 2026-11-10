package gacha_draw

import (
	"gorm.io/gorm"
)

var _ DrawWarehouseItemsModel = (*customDrawWarehouseItemsModel)(nil)

type (
	// DrawWarehouseItemsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDrawWarehouseItemsModel.
	DrawWarehouseItemsModel interface {
		drawWarehouseItemsModel
		customDrawWarehouseItemsLogicModel
	}

	customDrawWarehouseItemsModel struct {
		*defaultDrawWarehouseItemsModel
	}

	customDrawWarehouseItemsLogicModel interface {
	}
)

// NewDrawWarehouseItemsModel returns a model for the database table.
func NewDrawWarehouseItemsModel(conn *gorm.DB) DrawWarehouseItemsModel {
	return &customDrawWarehouseItemsModel{
		defaultDrawWarehouseItemsModel: newDrawWarehouseItemsModel(conn),
	}
}
