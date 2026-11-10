package gacha_thirdparty

import (
	"gorm.io/gorm"
)

var _ IntegrationMetricRawModel = (*customIntegrationMetricRawModel)(nil)

type (
	// IntegrationMetricRawModel is an interface to be customized, add more methods here,
	// and implement the added methods in customIntegrationMetricRawModel.
	IntegrationMetricRawModel interface {
		integrationMetricRawModel
		customIntegrationMetricRawLogicModel
	}

	customIntegrationMetricRawModel struct {
		*defaultIntegrationMetricRawModel
	}

	customIntegrationMetricRawLogicModel interface {
	}
)

// NewIntegrationMetricRawModel returns a model for the database table.
func NewIntegrationMetricRawModel(conn *gorm.DB) IntegrationMetricRawModel {
	return &customIntegrationMetricRawModel{
		defaultIntegrationMetricRawModel: newIntegrationMetricRawModel(conn),
	}
}
