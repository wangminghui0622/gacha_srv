package gacha_identity

import (
	"gorm.io/gorm"
)

var _ UserUsersModel = (*customUserUsersModel)(nil)

type (
	// UserUsersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserUsersModel.
	UserUsersModel interface {
		userUsersModel
		customUserUsersLogicModel
	}

	customUserUsersModel struct {
		*defaultUserUsersModel
	}

	customUserUsersLogicModel interface {
	}
)

// NewUserUsersModel returns a model for the database table.
func NewUserUsersModel(conn *gorm.DB) UserUsersModel {
	return &customUserUsersModel{
		defaultUserUsersModel: newUserUsersModel(conn),
	}
}
