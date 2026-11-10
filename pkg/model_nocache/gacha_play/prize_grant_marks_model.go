package gacha_play

import (
	"gorm.io/gorm"
)

var _ PrizeGrantMarksModel = (*customPrizeGrantMarksModel)(nil)

type (
	// PrizeGrantMarksModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPrizeGrantMarksModel.
	PrizeGrantMarksModel interface {
		prizeGrantMarksModel
		customPrizeGrantMarksLogicModel
	}

	customPrizeGrantMarksModel struct {
		*defaultPrizeGrantMarksModel
	}

	customPrizeGrantMarksLogicModel interface {
	}
)

// NewPrizeGrantMarksModel returns a model for the database table.
func NewPrizeGrantMarksModel(conn *gorm.DB) PrizeGrantMarksModel {
	return &customPrizeGrantMarksModel{
		defaultPrizeGrantMarksModel: newPrizeGrantMarksModel(conn),
	}
}
