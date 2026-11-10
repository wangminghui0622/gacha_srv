package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ CampaignSignInsModel = (*customCampaignSignInsModel)(nil)

type (
	// CampaignSignInsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCampaignSignInsModel.
	CampaignSignInsModel interface {
		campaignSignInsModel
		customCampaignSignInsLogicModel
	}

	customCampaignSignInsModel struct {
		*defaultCampaignSignInsModel
	}

	customCampaignSignInsLogicModel interface {
	}
)

// NewCampaignSignInsModel returns a model for the database table.
func NewCampaignSignInsModel(conn *gorm.DB) CampaignSignInsModel {
	return &customCampaignSignInsModel{
		defaultCampaignSignInsModel: newCampaignSignInsModel(conn),
	}
}
