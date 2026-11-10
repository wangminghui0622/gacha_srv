package gacha_money

import (
	"gorm.io/gorm"
)

var _ WalletAccountsModel = (*customWalletAccountsModel)(nil)

type (
	// WalletAccountsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customWalletAccountsModel.
	WalletAccountsModel interface {
		walletAccountsModel
		customWalletAccountsLogicModel
	}

	customWalletAccountsModel struct {
		*defaultWalletAccountsModel
	}

	customWalletAccountsLogicModel interface {
	}
)

// NewWalletAccountsModel returns a model for the database table.
func NewWalletAccountsModel(conn *gorm.DB) WalletAccountsModel {
	return &customWalletAccountsModel{
		defaultWalletAccountsModel: newWalletAccountsModel(conn),
	}
}
