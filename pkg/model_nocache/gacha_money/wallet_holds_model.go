package gacha_money

import (
	"gorm.io/gorm"
)

var _ WalletHoldsModel = (*customWalletHoldsModel)(nil)

type (
	// WalletHoldsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customWalletHoldsModel.
	WalletHoldsModel interface {
		walletHoldsModel
		customWalletHoldsLogicModel
	}

	customWalletHoldsModel struct {
		*defaultWalletHoldsModel
	}

	customWalletHoldsLogicModel interface {
	}
)

// NewWalletHoldsModel returns a model for the database table.
func NewWalletHoldsModel(conn *gorm.DB) WalletHoldsModel {
	return &customWalletHoldsModel{
		defaultWalletHoldsModel: newWalletHoldsModel(conn),
	}
}
