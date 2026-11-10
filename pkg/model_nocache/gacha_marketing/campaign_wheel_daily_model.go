package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ CampaignWheelDailyModel = (*customCampaignWheelDailyModel)(nil)

type (
	// CampaignWheelDailyModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCampaignWheelDailyModel.
	CampaignWheelDailyModel interface {
		campaignWheelDailyModel
		customCampaignWheelDailyLogicModel
	}

	customCampaignWheelDailyModel struct {
		*defaultCampaignWheelDailyModel
	}

	customCampaignWheelDailyLogicModel interface {
	}
)

// NewCampaignWheelDailyModel returns a model for the database table.
func NewCampaignWheelDailyModel(conn *gorm.DB) CampaignWheelDailyModel {
	return &customCampaignWheelDailyModel{
		defaultCampaignWheelDailyModel: newCampaignWheelDailyModel(conn),
	}
}
