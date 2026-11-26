package repo

import (
	"fmt"

	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/publicid"

	"gorm.io/gorm"
)

const publicIDBackfillBatch = 500

// BackfillUserPublicIDs 为历史用户补齐 public_id。
// 新用户由 UserUsers.AfterCreate 自动写入；本函数只处理 NULL/空串的旧行。
func BackfillUserPublicIDs(db *gorm.DB) error {
	var columnCount int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.COLUMNS " +
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_users' AND COLUMN_NAME = 'public_id'",
	).Scan(&columnCount).Error; err != nil {
		return fmt.Errorf("check user_users.public_id: %w", err)
	}
	if columnCount == 0 {
		return fmt.Errorf("user_users.public_id missing: apply script/split/02_gacha_identity.sql first")
	}

	var lastID int64
	for {
		var users []gacha_identity.UserUsers
		query := db.Where("public_id IS NULL OR public_id = ''")
		if lastID > 0 {
			query = query.Where("id > ?", lastID)
		}
		if err := query.Order("id ASC").Limit(publicIDBackfillBatch).Find(&users).Error; err != nil {
			return fmt.Errorf("list users without public_id: %w", err)
		}
		if len(users) == 0 {
			return nil
		}
		for i := range users {
			publicID, err := publicid.EncodeUserID(users[i].Id)
			if err != nil {
				return fmt.Errorf("encode user %d public id: %w", users[i].Id, err)
			}
			if err := db.Model(&gacha_identity.UserUsers{}).
				Where("id = ? AND (public_id IS NULL OR public_id = '')", users[i].Id).
				Update("public_id", publicID).Error; err != nil {
				return fmt.Errorf("backfill user %d public id: %w", users[i].Id, err)
			}
		}
		lastID = users[len(users)-1].Id
	}
}
