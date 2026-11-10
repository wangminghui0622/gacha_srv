package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ CampaignRedemptionUsersModel = (*customCampaignRedemptionUsersModel)(nil)

type (
	// CampaignRedemptionUsersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCampaignRedemptionUsersModel.
	CampaignRedemptionUsersModel interface {
		campaignRedemptionUsersModel
		customCampaignRedemptionUsersLogicModel
	}

	customCampaignRedemptionUsersModel struct {
		*defaultCampaignRedemptionUsersModel
	}

	customCampaignRedemptionUsersLogicModel interface {
	}
)

// NewCampaignRedemptionUsersModel returns a model for the database table.
func NewCampaignRedemptionUsersModel(conn *gorm.DB) CampaignRedemptionUsersModel {
	return &customCampaignRedemptionUsersModel{
		defaultCampaignRedemptionUsersModel: newCampaignRedemptionUsersModel(conn),
	}
}
