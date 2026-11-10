package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ UserCouponsModel = (*customUserCouponsModel)(nil)

type (
	// UserCouponsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserCouponsModel.
	UserCouponsModel interface {
		userCouponsModel
		customUserCouponsLogicModel
	}

	customUserCouponsModel struct {
		*defaultUserCouponsModel
	}

	customUserCouponsLogicModel interface {
	}
)

// NewUserCouponsModel returns a model for the database table.
func NewUserCouponsModel(conn *gorm.DB) UserCouponsModel {
	return &customUserCouponsModel{
		defaultUserCouponsModel: newUserCouponsModel(conn),
	}
}
