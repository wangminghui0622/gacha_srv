// 手写模型：reporting_metric_dimension 为联合主键（metric_code, dimension_code），
// goctl model 不支持联合主键，无法自动生成——风格对齐 goctl 生成物。
package gacha_ops

import (
	"context"

	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/SpectatorNan/gorm-zero/gormc/pagex"
	"gorm.io/gorm"
)

type (
	// ReportingMetricDimensionModel 指标×维度能力表（只读目录 + 校验命中）。
	ReportingMetricDimensionModel interface {
		Insert(ctx context.Context, tx *gorm.DB, data *ReportingMetricDimension) error
		BatchInsert(ctx context.Context, tx *gorm.DB, news []ReportingMetricDimension) error
		FindOne(ctx context.Context, metricCode string, dimensionCode string) (*ReportingMetricDimension, error)
		Delete(ctx context.Context, tx *gorm.DB, metricCode string, dimensionCode string) error
		FindPageList(ctx context.Context, page *pagex.ListReq, orderBy pagex.OrderBy,
			orderKeys map[string]string, whereClause func(db *gorm.DB) *gorm.DB) ([]ReportingMetricDimension, int64, error)
	}

	defaultReportingMetricDimensionModel struct {
		conn  *gorm.DB
		table string
	}

	ReportingMetricDimension struct {
		MetricCode    string `gorm:"column:metric_code;primary_key"`    // 指标编码
		DimensionCode string `gorm:"column:dimension_code;primary_key"` // 维度编码
	}
)

func (ReportingMetricDimension) TableName() string {
	return "`reporting_metric_dimension`"
}

// NewReportingMetricDimensionModel 返回模型（手写，对齐 goctl 风格）。
func NewReportingMetricDimensionModel(db *gorm.DB) ReportingMetricDimensionModel {
	return &defaultReportingMetricDimensionModel{conn: db, table: "`reporting_metric_dimension`"}
}

func (m *defaultReportingMetricDimensionModel) Insert(ctx context.Context, tx *gorm.DB, data *ReportingMetricDimension) error {
	db := m.conn
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Create(&data).Error
}

func (m *defaultReportingMetricDimensionModel) BatchInsert(ctx context.Context, tx *gorm.DB, news []ReportingMetricDimension) error {
	db := m.conn
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Create(&news).Error
}

func (m *defaultReportingMetricDimensionModel) FindOne(ctx context.Context, metricCode string, dimensionCode string) (*ReportingMetricDimension, error) {
	var resp ReportingMetricDimension
	err := m.conn.WithContext(ctx).Model(&ReportingMetricDimension{}).
		Where("`metric_code` = ? AND `dimension_code` = ?", metricCode, dimensionCode).Take(&resp).Error
	switch err {
	case nil:
		return &resp, nil
	case gormc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultReportingMetricDimensionModel) Delete(ctx context.Context, tx *gorm.DB, metricCode string, dimensionCode string) error {
	db := m.conn
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Where("`metric_code` = ? AND `dimension_code` = ?", metricCode, dimensionCode).
		Delete(&ReportingMetricDimension{}).Error
}

func (m *defaultReportingMetricDimensionModel) FindPageList(ctx context.Context, page *pagex.ListReq, orderBy pagex.OrderBy,
	orderKeys map[string]string, whereClause func(db *gorm.DB) *gorm.DB) ([]ReportingMetricDimension, int64, error) {
	conn := m.conn
	formatDB := func() (*gorm.DB, *gorm.DB) {
		db := conn.Model(&ReportingMetricDimension{})
		if whereClause != nil {
			db = whereClause(db)
		}
		return db, nil
	}
	return pagex.FindPageListWithCountMultiOrderBy[ReportingMetricDimension](ctx, page, []pagex.OrderBy{orderBy}, orderKeys, formatDB)
}
