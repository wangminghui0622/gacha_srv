package gacha_draw

import (
	"gorm.io/gorm"
)

var _ DtmBarrierModel = (*customDtmBarrierModel)(nil)

type (
	// DtmBarrierModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDtmBarrierModel.
	DtmBarrierModel interface {
		dtmBarrierModel
		customDtmBarrierLogicModel
	}

	customDtmBarrierModel struct {
		*defaultDtmBarrierModel
	}

	customDtmBarrierLogicModel interface {
	}
)

// NewDtmBarrierModel returns a model for the database table.
func NewDtmBarrierModel(conn *gorm.DB) DtmBarrierModel {
	return &customDtmBarrierModel{
		defaultDtmBarrierModel: newDtmBarrierModel(conn),
	}
}
