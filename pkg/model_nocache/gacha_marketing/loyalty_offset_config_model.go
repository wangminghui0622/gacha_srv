package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ LoyaltyOffsetConfigModel = (*customLoyaltyOffsetConfigModel)(nil)

type (
	// LoyaltyOffsetConfigModel is an interface to be customized, add more methods here,
	// and implement the added methods in customLoyaltyOffsetConfigModel.
	LoyaltyOffsetConfigModel interface {
		loyaltyOffsetConfigModel
		customLoyaltyOffsetConfigLogicModel
	}

	customLoyaltyOffsetConfigModel struct {
		*defaultLoyaltyOffsetConfigModel
	}

	customLoyaltyOffsetConfigLogicModel interface {
	}
)

// NewLoyaltyOffsetConfigModel returns a model for the database table.
func NewLoyaltyOffsetConfigModel(conn *gorm.DB) LoyaltyOffsetConfigModel {
	return &customLoyaltyOffsetConfigModel{
		defaultLoyaltyOffsetConfigModel: newLoyaltyOffsetConfigModel(conn),
	}
}
