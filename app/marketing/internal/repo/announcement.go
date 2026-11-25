package repo

import (
	"time"

	"gacha_srv/app/marketing/marketing"
	"gorm.io/gorm"
)

// AnnouncementRow announcements 表行。
type AnnouncementRow struct {
	ID         int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Title      string     `gorm:"column:title"`
	Type       int32      `gorm:"column:type"`
	PublishAt  *time.Time `gorm:"column:publish_at"`
	Visible    int32      `gorm:"column:visible"`
	JumpType   string     `gorm:"column:jump_type"`
	JumpTarget string     `gorm:"column:jump_target"`
	Content    string     `gorm:"column:content"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at"`
}

func (AnnouncementRow) TableName() string { return "announcements" }

func ListAnnouncements(tx *gorm.DB, offset, limit int) ([]AnnouncementRow, int64, error) {
	var total int64
	if err := tx.Model(&AnnouncementRow{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AnnouncementRow
	if err := tx.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func GetAnnouncement(tx *gorm.DB, id int64) (*AnnouncementRow, error) {
	var row AnnouncementRow
	if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func InsertAnnouncement(tx *gorm.DB, row *AnnouncementRow) error {
	return tx.Create(row).Error
}

func UpdateAnnouncement(tx *gorm.DB, row *AnnouncementRow) error {
	return tx.Model(&AnnouncementRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"title":       row.Title,
		"type":        row.Type,
		"publish_at":  row.PublishAt,
		"visible":     row.Visible,
		"jump_type":   row.JumpType,
		"jump_target": row.JumpTarget,
		"content":     row.Content,
	}).Error
}

func (r *AnnouncementRow) ToProto() *marketing.Announcement {
	publishAt := int64(0)
	if r.PublishAt != nil {
		publishAt = r.PublishAt.UnixMilli()
	}
	return &marketing.Announcement{
		Id:                  uint64(r.ID),
		Title:               r.Title,
		Type:                r.Type,
		PublishAtUnixMillis: publishAt,
		Visible:             r.Visible == 1,
		JumpType:            r.JumpType,
		JumpTarget:          r.JumpTarget,
		Content:             r.Content,
		CreatedAtUnixMillis: timeToUnixMillis(r.CreatedAt),
		UpdatedAtUnixMillis: timeToUnixMillis(r.UpdatedAt),
	}
}

func AnnouncementFromProto(a *marketing.Announcement) *AnnouncementRow {
	if a == nil {
		return nil
	}
	return &AnnouncementRow{
		ID:         int64(a.Id),
		Title:      a.Title,
		Type:       a.Type,
		PublishAt:  unixMillisTime(a.PublishAtUnixMillis),
		Visible:    boolToInt32(a.Visible),
		JumpType:   a.JumpType,
		JumpTarget: a.JumpTarget,
		Content:    a.Content,
	}
}
