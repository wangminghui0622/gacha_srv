package gacha_identity

import (
	"context"
	"fmt"

	"gacha_srv/pkg/utils/publicid"

	"github.com/SpectatorNan/gorm-zero/gormc"
	"gorm.io/gorm"
)

// AfterCreate 新建用户后持久化对外字符串 ID。
// public_id 由自增主键编码而来，只能在 INSERT 拿到 id 后回写；
// 使用同一个 tx，回写失败会连同用户 INSERT 一起回滚。
// FindOneByPublicId 按对外字符串 ID 查询用户。
func (m *customUserUsersModel) FindOneByPublicId(ctx context.Context, publicId string) (*UserUsers, error) {
	var resp UserUsers
	err := m.conn.WithContext(ctx).Model(&UserUsers{}).Where("public_id = ?", publicId).Take(&resp).Error
	switch err {
	case nil:
		return &resp, nil
	case gormc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (u *UserUsers) AfterCreate(tx *gorm.DB) error {
	if u.PublicId != nil && *u.PublicId != "" {
		return nil
	}
	publicID, err := publicid.EncodeUserID(u.Id)
	if err != nil {
		return fmt.Errorf("generate user public id: %w", err)
	}
	if err := tx.Model(&UserUsers{}).
		Where("id = ?", u.Id).
		Update("public_id", publicID).Error; err != nil {
		return fmt.Errorf("persist user public id: %w", err)
	}
	u.PublicId = &publicID
	return nil
}
