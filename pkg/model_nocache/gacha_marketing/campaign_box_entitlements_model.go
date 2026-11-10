package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ CampaignBoxEntitlementsModel = (*customCampaignBoxEntitlementsModel)(nil)

type (
	// CampaignBoxEntitlementsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCampaignBoxEntitlementsModel.
	CampaignBoxEntitlementsModel interface {
		campaignBoxEntitlementsModel
		customCampaignBoxEntitlementsLogicModel
	}

	customCampaignBoxEntitlementsModel struct {
		*defaultCampaignBoxEntitlementsModel
	}

	customCampaignBoxEntitlementsLogicModel interface {
	}
)

// NewCampaignBoxEntitlementsModel returns a model for the database table.
func NewCampaignBoxEntitlementsModel(conn *gorm.DB) CampaignBoxEntitlementsModel {
	return &customCampaignBoxEntitlementsModel{
		defaultCampaignBoxEntitlementsModel: newCampaignBoxEntitlementsModel(conn),
	}
}
