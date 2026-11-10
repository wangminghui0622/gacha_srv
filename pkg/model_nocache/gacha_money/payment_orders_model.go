package gacha_money

import (
	"gorm.io/gorm"
)

var _ PaymentOrdersModel = (*customPaymentOrdersModel)(nil)

type (
	// PaymentOrdersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPaymentOrdersModel.
	PaymentOrdersModel interface {
		paymentOrdersModel
		customPaymentOrdersLogicModel
	}

	customPaymentOrdersModel struct {
		*defaultPaymentOrdersModel
	}

	customPaymentOrdersLogicModel interface {
	}
)

// NewPaymentOrdersModel returns a model for the database table.
func NewPaymentOrdersModel(conn *gorm.DB) PaymentOrdersModel {
	return &customPaymentOrdersModel{
		defaultPaymentOrdersModel: newPaymentOrdersModel(conn),
	}
}
