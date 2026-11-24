package repo

import (
	"context"

	"gorm.io/gorm"
)

// ListMemberUserIDs 按累计消费区间分页列出会员用户ID（user_id 升序，游标分页）。
func ListMemberUserIDs(ctx context.Context, db *gorm.DB, minSpendMinor, maxSpendMinor, afterUserID int64, limit int) ([]int64, int64, error) {
	var ids []int64
	q := db.WithContext(ctx).Table("task_member").
		Where("spend_minor >= ?", minSpendMinor).
		Where("user_id > ?", afterUserID)
	if maxSpendMinor > 0 {
		q = q.Where("spend_minor <= ?", maxSpendMinor)
	}
	if err := q.Order("user_id ASC").Limit(limit).Pluck("user_id", &ids).Error; err != nil {
		return nil, 0, err
	}
	var next int64
	if len(ids) == limit {
		next = ids[len(ids)-1]
	}
	return ids, next, nil
}
