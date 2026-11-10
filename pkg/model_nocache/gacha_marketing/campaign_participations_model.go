package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ CampaignParticipationsModel = (*customCampaignParticipationsModel)(nil)

type (
	// CampaignParticipationsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCampaignParticipationsModel.
	CampaignParticipationsModel interface {
		campaignParticipationsModel
		customCampaignParticipationsLogicModel
	}

	customCampaignParticipationsModel struct {
		*defaultCampaignParticipationsModel
	}

	customCampaignParticipationsLogicModel interface {
	}
)

// NewCampaignParticipationsModel returns a model for the database table.
func NewCampaignParticipationsModel(conn *gorm.DB) CampaignParticipationsModel {
	return &customCampaignParticipationsModel{
		defaultCampaignParticipationsModel: newCampaignParticipationsModel(conn),
	}
}
