package gacha_identity

import (
	"gorm.io/gorm"
)

var _ UserThirdPartiesModel = (*customUserThirdPartiesModel)(nil)

type (
	// UserThirdPartiesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserThirdPartiesModel.
	UserThirdPartiesModel interface {
		userThirdPartiesModel
		customUserThirdPartiesLogicModel
	}

	customUserThirdPartiesModel struct {
		*defaultUserThirdPartiesModel
	}

	customUserThirdPartiesLogicModel interface {
	}
)

// NewUserThirdPartiesModel returns a model for the database table.
func NewUserThirdPartiesModel(conn *gorm.DB) UserThirdPartiesModel {
	return &customUserThirdPartiesModel{
		defaultUserThirdPartiesModel: newUserThirdPartiesModel(conn),
	}
}
