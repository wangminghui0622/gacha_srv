package gacha_identity

import (
	"context"

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
		// FindOneByPublicId 按对外字符串 ID 查询（uk_public_id）
		FindOneByPublicId(ctx context.Context, publicId string) (*UserUsers, error)
	}
)

// NewUserUsersModel returns a model for the database table.
func NewUserUsersModel(conn *gorm.DB) UserUsersModel {
	return &customUserUsersModel{
		defaultUserUsersModel: newUserUsersModel(conn),
	}
}
