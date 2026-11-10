package gacha_money

import (
	"gorm.io/gorm"
)

var _ PaymentLegsModel = (*customPaymentLegsModel)(nil)

type (
	// PaymentLegsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPaymentLegsModel.
	PaymentLegsModel interface {
		paymentLegsModel
		customPaymentLegsLogicModel
	}

	customPaymentLegsModel struct {
		*defaultPaymentLegsModel
	}

	customPaymentLegsLogicModel interface {
	}
)

// NewPaymentLegsModel returns a model for the database table.
func NewPaymentLegsModel(conn *gorm.DB) PaymentLegsModel {
	return &customPaymentLegsModel{
		defaultPaymentLegsModel: newPaymentLegsModel(conn),
	}
}
