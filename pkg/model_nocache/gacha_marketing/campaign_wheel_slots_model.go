package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ CampaignWheelSlotsModel = (*customCampaignWheelSlotsModel)(nil)

type (
	// CampaignWheelSlotsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCampaignWheelSlotsModel.
	CampaignWheelSlotsModel interface {
		campaignWheelSlotsModel
		customCampaignWheelSlotsLogicModel
	}

	customCampaignWheelSlotsModel struct {
		*defaultCampaignWheelSlotsModel
	}

	customCampaignWheelSlotsLogicModel interface {
	}
)

// NewCampaignWheelSlotsModel returns a model for the database table.
func NewCampaignWheelSlotsModel(conn *gorm.DB) CampaignWheelSlotsModel {
	return &customCampaignWheelSlotsModel{
		defaultCampaignWheelSlotsModel: newCampaignWheelSlotsModel(conn),
	}
}
