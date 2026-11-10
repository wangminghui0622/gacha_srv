package gacha_ops

import (
	"gorm.io/gorm"
)

var _ RolePermissionsModel = (*customRolePermissionsModel)(nil)

type (
	// RolePermissionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRolePermissionsModel.
	RolePermissionsModel interface {
		rolePermissionsModel
		customRolePermissionsLogicModel
	}

	customRolePermissionsModel struct {
		*defaultRolePermissionsModel
	}

	customRolePermissionsLogicModel interface {
	}
)

// NewRolePermissionsModel returns a model for the database table.
func NewRolePermissionsModel(conn *gorm.DB) RolePermissionsModel {
	return &customRolePermissionsModel{
		defaultRolePermissionsModel: newRolePermissionsModel(conn),
	}
}
