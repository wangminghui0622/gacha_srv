package repo

import (
	"time"

	"gacha_srv/app/marketing/marketing"
	"gorm.io/gorm"
)

// BannerRow banners 表行。
type BannerRow struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Position        int32     `gorm:"column:position"`
	BannerType      string    `gorm:"column:banner_type"`
	Name            string    `gorm:"column:name"`
	ImageURL        string    `gorm:"column:image_url"`
	Visible         int32     `gorm:"column:visible"`
	TargetPoolID    int64     `gorm:"column:target_pool_id"`
	PopupType       string    `gorm:"column:popup_type"`
	PopupTargetType string    `gorm:"column:popup_target_type"`
	PopupTargetID   string    `gorm:"column:popup_target_id"`
	PopupContent    string    `gorm:"column:popup_content"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (BannerRow) TableName() string { return "banners" }

func ListBanners(tx *gorm.DB, offset, limit int) ([]BannerRow, int64, error) {
	var total int64
	if err := tx.Model(&BannerRow{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []BannerRow
	if err := tx.Order("position ASC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func GetBanner(tx *gorm.DB, id int64) (*BannerRow, error) {
	var row BannerRow
	if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func InsertBanner(tx *gorm.DB, row *BannerRow) error {
	return tx.Create(row).Error
}

func UpdateBanner(tx *gorm.DB, row *BannerRow) error {
	return tx.Model(&BannerRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"position":          row.Position,
		"banner_type":       row.BannerType,
		"name":              row.Name,
		"image_url":         row.ImageURL,
		"visible":           row.Visible,
		"target_pool_id":    row.TargetPoolID,
		"popup_type":        row.PopupType,
		"popup_target_type": row.PopupTargetType,
		"popup_target_id":   row.PopupTargetID,
		"popup_content":     row.PopupContent,
	}).Error
}

func (r *BannerRow) ToProto() *marketing.Banner {
	return &marketing.Banner{
		Id:                  uint64(r.ID),
		Position:            r.Position,
		Type:                r.BannerType,
		Name:                r.Name,
		ImageUrl:            r.ImageURL,
		Visible:             r.Visible == 1,
		TargetPoolId:        r.TargetPoolID,
		PopupType:           r.PopupType,
		PopupTargetType:     r.PopupTargetType,
		PopupTargetId:       r.PopupTargetID,
		PopupContent:        r.PopupContent,
		CreatedAtUnixMillis: timeToUnixMillis(r.CreatedAt),
		UpdatedAtUnixMillis: timeToUnixMillis(r.UpdatedAt),
	}
}

func BannerFromProto(b *marketing.Banner) *BannerRow {
	if b == nil {
		return nil
	}
	return &BannerRow{
		ID:              int64(b.Id),
		Position:        b.Position,
		BannerType:      b.Type,
		Name:            b.Name,
		ImageURL:        b.ImageUrl,
		Visible:         boolToInt32(b.Visible),
		TargetPoolID:    b.TargetPoolId,
		PopupType:       b.PopupType,
		PopupTargetType: b.PopupTargetType,
		PopupTargetID:   b.PopupTargetId,
		PopupContent:    b.PopupContent,
	}
}
