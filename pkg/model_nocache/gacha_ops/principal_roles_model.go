package gacha_ops

import (
	"gorm.io/gorm"
)

var _ PrincipalRolesModel = (*customPrincipalRolesModel)(nil)

type (
	// PrincipalRolesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPrincipalRolesModel.
	PrincipalRolesModel interface {
		principalRolesModel
		customPrincipalRolesLogicModel
	}

	customPrincipalRolesModel struct {
		*defaultPrincipalRolesModel
	}

	customPrincipalRolesLogicModel interface {
	}
)

// NewPrincipalRolesModel returns a model for the database table.
func NewPrincipalRolesModel(conn *gorm.DB) PrincipalRolesModel {
	return &customPrincipalRolesModel{
		defaultPrincipalRolesModel: newPrincipalRolesModel(conn),
	}
}
