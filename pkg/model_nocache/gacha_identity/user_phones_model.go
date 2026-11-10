package gacha_identity

import (
	"gorm.io/gorm"
)

var _ UserPhonesModel = (*customUserPhonesModel)(nil)

type (
	// UserPhonesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserPhonesModel.
	UserPhonesModel interface {
		userPhonesModel
		customUserPhonesLogicModel
	}

	customUserPhonesModel struct {
		*defaultUserPhonesModel
	}

	customUserPhonesLogicModel interface {
	}
)

// NewUserPhonesModel returns a model for the database table.
func NewUserPhonesModel(conn *gorm.DB) UserPhonesModel {
	return &customUserPhonesModel{
		defaultUserPhonesModel: newUserPhonesModel(conn),
	}
}
