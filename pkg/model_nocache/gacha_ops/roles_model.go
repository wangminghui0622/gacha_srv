package gacha_ops

import (
	"gorm.io/gorm"
)

var _ RolesModel = (*customRolesModel)(nil)

type (
	// RolesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRolesModel.
	RolesModel interface {
		rolesModel
		customRolesLogicModel
	}

	customRolesModel struct {
		*defaultRolesModel
	}

	customRolesLogicModel interface {
	}
)

// NewRolesModel returns a model for the database table.
func NewRolesModel(conn *gorm.DB) RolesModel {
	return &customRolesModel{
		defaultRolesModel: newRolesModel(conn),
	}
}
