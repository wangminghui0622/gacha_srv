package gacha_money

import (
	"gorm.io/gorm"
)

var _ PickupCodesModel = (*customPickupCodesModel)(nil)

type (
	// PickupCodesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPickupCodesModel.
	PickupCodesModel interface {
		pickupCodesModel
		customPickupCodesLogicModel
	}

	customPickupCodesModel struct {
		*defaultPickupCodesModel
	}

	customPickupCodesLogicModel interface {
	}
)

// NewPickupCodesModel returns a model for the database table.
func NewPickupCodesModel(conn *gorm.DB) PickupCodesModel {
	return &customPickupCodesModel{
		defaultPickupCodesModel: newPickupCodesModel(conn),
	}
}
