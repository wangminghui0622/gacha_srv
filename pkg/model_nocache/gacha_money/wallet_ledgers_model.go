package gacha_money

import (
	"gorm.io/gorm"
)

var _ WalletLedgersModel = (*customWalletLedgersModel)(nil)

type (
	// WalletLedgersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customWalletLedgersModel.
	WalletLedgersModel interface {
		walletLedgersModel
		customWalletLedgersLogicModel
	}

	customWalletLedgersModel struct {
		*defaultWalletLedgersModel
	}

	customWalletLedgersLogicModel interface {
	}
)

// NewWalletLedgersModel returns a model for the database table.
func NewWalletLedgersModel(conn *gorm.DB) WalletLedgersModel {
	return &customWalletLedgersModel{
		defaultWalletLedgersModel: newWalletLedgersModel(conn),
	}
}
