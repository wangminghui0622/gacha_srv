package gacha_draw

import (
	"gorm.io/gorm"
)

var _ DrawRecordsModel = (*customDrawRecordsModel)(nil)

type (
	// DrawRecordsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDrawRecordsModel.
	DrawRecordsModel interface {
		drawRecordsModel
		customDrawRecordsLogicModel
	}

	customDrawRecordsModel struct {
		*defaultDrawRecordsModel
	}

	customDrawRecordsLogicModel interface {
	}
)

// NewDrawRecordsModel returns a model for the database table.
func NewDrawRecordsModel(conn *gorm.DB) DrawRecordsModel {
	return &customDrawRecordsModel{
		defaultDrawRecordsModel: newDrawRecordsModel(conn),
	}
}
