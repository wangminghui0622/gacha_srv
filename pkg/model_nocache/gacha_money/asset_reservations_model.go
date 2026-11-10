package gacha_money

import (
	"gorm.io/gorm"
)

var _ AssetReservationsModel = (*customAssetReservationsModel)(nil)

type (
	// AssetReservationsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAssetReservationsModel.
	AssetReservationsModel interface {
		assetReservationsModel
		customAssetReservationsLogicModel
	}

	customAssetReservationsModel struct {
		*defaultAssetReservationsModel
	}

	customAssetReservationsLogicModel interface {
	}
)

// NewAssetReservationsModel returns a model for the database table.
func NewAssetReservationsModel(conn *gorm.DB) AssetReservationsModel {
	return &customAssetReservationsModel{
		defaultAssetReservationsModel: newAssetReservationsModel(conn),
	}
}
