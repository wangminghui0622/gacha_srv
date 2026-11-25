package repo

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserProfile 用户后台资料（头像/状态/吉祥展示/返佣比例/身份）。
type UserProfile struct {
	ID                int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID            int64     `gorm:"column:user_id"`
	AvatarURL         string    `gorm:"column:avatar_url"`
	Status            int32     `gorm:"column:status"`
	LuckyDisplay      string    `gorm:"column:lucky_display"`
	RegisterIP        string    `gorm:"column:register_ip"`
	RegisterDevice    string    `gorm:"column:register_device"`
	RebateRate        int32     `gorm:"column:rebate_rate"`
	Identity          string    `gorm:"column:identity"`
	ParentUserID      int64     `gorm:"column:parent_user_id"`
	ReissueLuckPoints int32     `gorm:"column:reissue_luck_points"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (UserProfile) TableName() string { return "user_profiles" }

// FetchUserProfiles 批量读取用户资料。
func FetchUserProfiles(db *gorm.DB, userIDs []int64) (map[int64]UserProfile, error) {
	result := make(map[int64]UserProfile, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	var rows []UserProfile
	if err := db.Where("user_id IN ?", userIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = row
	}
	return result, nil
}

// SaveUserProfile 按 user_id upsert 用户资料。
func SaveUserProfile(db *gorm.DB, profile *UserProfile) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"avatar_url", "status", "lucky_display", "register_ip",
			"register_device", "rebate_rate", "identity", "parent_user_id",
			"reissue_luck_points", "updated_at",
		}),
	}).Create(profile).Error
}
