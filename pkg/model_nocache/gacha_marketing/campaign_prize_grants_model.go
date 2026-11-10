package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ CampaignPrizeGrantsModel = (*customCampaignPrizeGrantsModel)(nil)

type (
	// CampaignPrizeGrantsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCampaignPrizeGrantsModel.
	CampaignPrizeGrantsModel interface {
		campaignPrizeGrantsModel
		customCampaignPrizeGrantsLogicModel
	}

	customCampaignPrizeGrantsModel struct {
		*defaultCampaignPrizeGrantsModel
	}

	customCampaignPrizeGrantsLogicModel interface {
	}
)

// NewCampaignPrizeGrantsModel returns a model for the database table.
func NewCampaignPrizeGrantsModel(conn *gorm.DB) CampaignPrizeGrantsModel {
	return &customCampaignPrizeGrantsModel{
		defaultCampaignPrizeGrantsModel: newCampaignPrizeGrantsModel(conn),
	}
}
