package repo

import (
	"encoding/json"
	"time"

	"gacha_srv/app/marketing/marketing"
	"gorm.io/gorm"
)

// ActivityRow activities 表行（活动列表）。
type ActivityRow struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement"`
	ActivityKey  string     `gorm:"column:activity_key"`
	Name         string     `gorm:"column:name"`
	Description  string     `gorm:"column:description"`
	Protocol     string     `gorm:"column:protocol"`
	Enabled      int32      `gorm:"column:enabled"`
	StartAt      *time.Time `gorm:"column:start_at"`
	EndAt        *time.Time `gorm:"column:end_at"`
	DailyStart   string     `gorm:"column:daily_start"`
	DailyEnd     string     `gorm:"column:daily_end"`
	ImagesJSON   string     `gorm:"column:images_json"`
	ImageMapJSON string     `gorm:"column:image_map_json"`
	ParamsJSON   string     `gorm:"column:params_json"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

func (ActivityRow) TableName() string { return "activities" }

func ListActivities(tx *gorm.DB, keyword string, offset, limit int) ([]ActivityRow, int64, error) {
	query := tx.Model(&ActivityRow{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("activity_key LIKE ? OR name LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ActivityRow
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func GetActivity(tx *gorm.DB, id int64) (*ActivityRow, error) {
	var row ActivityRow
	if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func ActivityKeyExists(tx *gorm.DB, key string, excludeID int64) (bool, error) {
	query := tx.Model(&ActivityRow{}).Where("activity_key = ?", key)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func InsertActivity(tx *gorm.DB, row *ActivityRow) error {
	return tx.Create(row).Error
}

func UpdateActivity(tx *gorm.DB, row *ActivityRow) error {
	return tx.Model(&ActivityRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"activity_key":   row.ActivityKey,
		"name":           row.Name,
		"description":    row.Description,
		"protocol":       row.Protocol,
		"enabled":        row.Enabled,
		"start_at":       row.StartAt,
		"end_at":         row.EndAt,
		"daily_start":    row.DailyStart,
		"daily_end":      row.DailyEnd,
		"images_json":    row.ImagesJSON,
		"image_map_json": row.ImageMapJSON,
		"params_json":    row.ParamsJSON,
	}).Error
}

func MarshalActivityImages(items []string) string {
	if items == nil {
		return "[]"
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func UnmarshalActivityImages(raw string) []string {
	items := []string{}
	if raw == "" {
		return items
	}
	_ = json.Unmarshal([]byte(raw), &items)
	return items
}

func MarshalActivityMap(values map[string]string) string {
	if values == nil {
		return "{}"
	}
	b, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func UnmarshalActivityMap(raw string) map[string]string {
	values := map[string]string{}
	if raw == "" {
		return values
	}
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

// ToProto 活动行 → console Activity。
func (r *ActivityRow) ToProto() *marketing.Activity {
	startAt := int64(0)
	if r.StartAt != nil {
		startAt = r.StartAt.UnixMilli()
	}
	endAt := int64(0)
	if r.EndAt != nil {
		endAt = r.EndAt.UnixMilli()
	}
	return &marketing.Activity{
		Id:                  uint64(r.ID),
		ActivityKey:         r.ActivityKey,
		Name:                r.Name,
		Description:         r.Description,
		Protocol:            r.Protocol,
		Enabled:             r.Enabled == 1,
		StartAtUnixMillis:   startAt,
		EndAtUnixMillis:     endAt,
		DailyStart:          r.DailyStart,
		DailyEnd:            r.DailyEnd,
		Images:              UnmarshalActivityImages(r.ImagesJSON),
		ImageMap:            UnmarshalActivityMap(r.ImageMapJSON),
		Params:              UnmarshalActivityMap(r.ParamsJSON),
		CreatedAtUnixMillis: timeToUnixMillis(r.CreatedAt),
		UpdatedAtUnixMillis: timeToUnixMillis(r.UpdatedAt),
	}
}

// ActivityFromProto proto → 表行（创建/编辑共用）。
func ActivityFromProto(a *marketing.Activity) *ActivityRow {
	if a == nil {
		return nil
	}
	return &ActivityRow{
		ID:           int64(a.Id),
		ActivityKey:  a.ActivityKey,
		Name:         a.Name,
		Description:  a.Description,
		Protocol:     a.Protocol,
		Enabled:      boolToInt32(a.Enabled),
		StartAt:      unixMillisTime(a.StartAtUnixMillis),
		EndAt:        unixMillisTime(a.EndAtUnixMillis),
		DailyStart:   a.DailyStart,
		DailyEnd:     a.DailyEnd,
		ImagesJSON:   MarshalActivityImages(a.Images),
		ImageMapJSON: MarshalActivityMap(a.ImageMap),
		ParamsJSON:   MarshalActivityMap(a.Params),
	}
}
