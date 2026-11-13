package repo

import (
	"context"

	"gorm.io/gorm"
)

type PrizeGradeRow struct {
	Sort     int64  `gorm:"column:sort"`
	Code     string `gorm:"column:code"`
	Category string `gorm:"column:category"`
}

type PrizeGradeStore struct {
	db *gorm.DB
}

func NewPrizeGradeStore(db *gorm.DB) *PrizeGradeStore {
	return &PrizeGradeStore{db: db}
}

// List 按排序值倒序返回赏级配置。
func (s *PrizeGradeStore) List(ctx context.Context) ([]PrizeGradeRow, error) {
	var rows []PrizeGradeRow
	err := s.db.WithContext(ctx).Table("prize_grades").
		Select("sort, code, category").
		Order("sort DESC, id").
		Find(&rows).Error
	return rows, err
}
