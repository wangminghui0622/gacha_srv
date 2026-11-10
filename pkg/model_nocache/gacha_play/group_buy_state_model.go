package gacha_play

import (
	"gorm.io/gorm"
)

var _ GroupBuyStateModel = (*customGroupBuyStateModel)(nil)

type (
	// GroupBuyStateModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGroupBuyStateModel.
	GroupBuyStateModel interface {
		groupBuyStateModel
		customGroupBuyStateLogicModel
	}

	customGroupBuyStateModel struct {
		*defaultGroupBuyStateModel
	}

	customGroupBuyStateLogicModel interface {
	}
)

// NewGroupBuyStateModel returns a model for the database table.
func NewGroupBuyStateModel(conn *gorm.DB) GroupBuyStateModel {
	return &customGroupBuyStateModel{
		defaultGroupBuyStateModel: newGroupBuyStateModel(conn),
	}
}
