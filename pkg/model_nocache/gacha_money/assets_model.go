package gacha_money

import (
	"gorm.io/gorm"
)

var _ AssetsModel = (*customAssetsModel)(nil)

type (
	// AssetsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAssetsModel.
	AssetsModel interface {
		assetsModel
		customAssetsLogicModel
	}

	customAssetsModel struct {
		*defaultAssetsModel
	}

	customAssetsLogicModel interface {
	}
)

// NewAssetsModel returns a model for the database table.
func NewAssetsModel(conn *gorm.DB) AssetsModel {
	return &customAssetsModel{
		defaultAssetsModel: newAssetsModel(conn),
	}
}
