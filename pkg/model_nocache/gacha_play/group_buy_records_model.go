package gacha_play

import (
	"gorm.io/gorm"
)

var _ GroupBuyRecordsModel = (*customGroupBuyRecordsModel)(nil)

type (
	// GroupBuyRecordsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGroupBuyRecordsModel.
	GroupBuyRecordsModel interface {
		groupBuyRecordsModel
		customGroupBuyRecordsLogicModel
	}

	customGroupBuyRecordsModel struct {
		*defaultGroupBuyRecordsModel
	}

	customGroupBuyRecordsLogicModel interface {
	}
)

// NewGroupBuyRecordsModel returns a model for the database table.
func NewGroupBuyRecordsModel(conn *gorm.DB) GroupBuyRecordsModel {
	return &customGroupBuyRecordsModel{
		defaultGroupBuyRecordsModel: newGroupBuyRecordsModel(conn),
	}
}
