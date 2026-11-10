package gacha_identity

import (
	"gorm.io/gorm"
)

var _ UserPhoneGramsModel = (*customUserPhoneGramsModel)(nil)

type (
	// UserPhoneGramsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserPhoneGramsModel.
	UserPhoneGramsModel interface {
		userPhoneGramsModel
		customUserPhoneGramsLogicModel
	}

	customUserPhoneGramsModel struct {
		*defaultUserPhoneGramsModel
	}

	customUserPhoneGramsLogicModel interface {
	}
)

// NewUserPhoneGramsModel returns a model for the database table.
func NewUserPhoneGramsModel(conn *gorm.DB) UserPhoneGramsModel {
	return &customUserPhoneGramsModel{
		defaultUserPhoneGramsModel: newUserPhoneGramsModel(conn),
	}
}
