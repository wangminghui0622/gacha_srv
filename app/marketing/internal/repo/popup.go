package repo

import (
	"time"

	"gacha_srv/app/marketing/marketing"
	"gorm.io/gorm"
)

// PopupRow popups 表行。
type PopupRow struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ActiveDisplay int32     `gorm:"column:active_display"`
	Status        string    `gorm:"column:status"`
	SortOrder     int32     `gorm:"column:sort_order"`
	Name          string    `gorm:"column:name"`
	PopupType     string    `gorm:"column:popup_type"`
	ConditionText string    `gorm:"column:condition_text"`
	JumpType      string    `gorm:"column:jump_type"`
	JumpTarget    string    `gorm:"column:jump_target"`
	ImageURL      string    `gorm:"column:image_url"`
	Content       string    `gorm:"column:content"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (PopupRow) TableName() string { return "popups" }

func ListPopups(tx *gorm.DB, offset, limit int) ([]PopupRow, int64, error) {
	var total int64
	if err := tx.Model(&PopupRow{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []PopupRow
	if err := tx.Order("sort_order ASC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func GetPopup(tx *gorm.DB, id int64) (*PopupRow, error) {
	var row PopupRow
	if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func InsertPopup(tx *gorm.DB, row *PopupRow) error {
	return tx.Create(row).Error
}

func UpdatePopup(tx *gorm.DB, row *PopupRow) error {
	return tx.Model(&PopupRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"active_display": row.ActiveDisplay,
		"status":         row.Status,
		"sort_order":     row.SortOrder,
		"name":           row.Name,
		"popup_type":     row.PopupType,
		"condition_text": row.ConditionText,
		"jump_type":      row.JumpType,
		"jump_target":    row.JumpTarget,
		"image_url":      row.ImageURL,
		"content":        row.Content,
	}).Error
}

func (r *PopupRow) ToProto() *marketing.Popup {
	return &marketing.Popup{
		Id:                  uint64(r.ID),
		ActiveDisplay:       r.ActiveDisplay == 1,
		Status:              r.Status,
		SortOrder:           r.SortOrder,
		Name:                r.Name,
		Type:                r.PopupType,
		Condition:           r.ConditionText,
		JumpType:            r.JumpType,
		JumpTarget:          r.JumpTarget,
		ImageUrl:            r.ImageURL,
		Content:             r.Content,
		CreatedAtUnixMillis: timeToUnixMillis(r.CreatedAt),
		UpdatedAtUnixMillis: timeToUnixMillis(r.UpdatedAt),
	}
}

func PopupFromProto(p *marketing.Popup) *PopupRow {
	if p == nil {
		return nil
	}
	return &PopupRow{
		ID:            int64(p.Id),
		ActiveDisplay: boolToInt32(p.ActiveDisplay),
		Status:        p.Status,
		SortOrder:     p.SortOrder,
		Name:          p.Name,
		PopupType:     p.Type,
		ConditionText: p.Condition,
		JumpType:      p.JumpType,
		JumpTarget:    p.JumpTarget,
		ImageURL:      p.ImageUrl,
		Content:       p.Content,
	}
}
