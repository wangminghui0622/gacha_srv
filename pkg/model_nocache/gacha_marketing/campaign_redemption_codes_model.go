package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ CampaignRedemptionCodesModel = (*customCampaignRedemptionCodesModel)(nil)

type (
	// CampaignRedemptionCodesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCampaignRedemptionCodesModel.
	CampaignRedemptionCodesModel interface {
		campaignRedemptionCodesModel
		customCampaignRedemptionCodesLogicModel
	}

	customCampaignRedemptionCodesModel struct {
		*defaultCampaignRedemptionCodesModel
	}

	customCampaignRedemptionCodesLogicModel interface {
	}
)

// NewCampaignRedemptionCodesModel returns a model for the database table.
func NewCampaignRedemptionCodesModel(conn *gorm.DB) CampaignRedemptionCodesModel {
	return &customCampaignRedemptionCodesModel{
		defaultCampaignRedemptionCodesModel: newCampaignRedemptionCodesModel(conn),
	}
}
