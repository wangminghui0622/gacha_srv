package gacha_money

import (
	"gorm.io/gorm"
)

var _ PaymentRefundsModel = (*customPaymentRefundsModel)(nil)

type (
	// PaymentRefundsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPaymentRefundsModel.
	PaymentRefundsModel interface {
		paymentRefundsModel
		customPaymentRefundsLogicModel
	}

	customPaymentRefundsModel struct {
		*defaultPaymentRefundsModel
	}

	customPaymentRefundsLogicModel interface {
	}
)

// NewPaymentRefundsModel returns a model for the database table.
func NewPaymentRefundsModel(conn *gorm.DB) PaymentRefundsModel {
	return &customPaymentRefundsModel{
		defaultPaymentRefundsModel: newPaymentRefundsModel(conn),
	}
}
