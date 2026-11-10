package logic

import (
	"context"

	drawmodel "gacha_srv/pkg/model/gacha_draw"

	"gorm.io/gorm"
)

// boxNames 宝箱名映射（静态配置、数量少，一次取全）
func boxNames(ctx context.Context, db *gorm.DB) (map[int64]string, error) {
	var boxes []drawmodel.DrawBoxes
	if err := db.WithContext(ctx).Find(&boxes).Error; err != nil {
		return nil, err
	}
	m := make(map[int64]string, len(boxes))
	for _, b := range boxes {
		m[b.Id] = b.BoxName
	}
	return m, nil
}
