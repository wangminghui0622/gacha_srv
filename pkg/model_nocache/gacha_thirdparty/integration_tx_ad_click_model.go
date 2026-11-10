package gacha_thirdparty

import (
	"context"

	"gorm.io/gorm"
)

var _ IntegrationTxAdClickModel = (*customIntegrationTxAdClickModel)(nil)

type (
	// IntegrationTxAdClickModel is an interface to be customized, add more methods here,
	// and implement the added methods in customIntegrationTxAdClickModel.
	IntegrationTxAdClickModel interface {
		integrationTxAdClickModel
		customIntegrationTxAdClickLogicModel
	}

	customIntegrationTxAdClickModel struct {
		*defaultIntegrationTxAdClickModel
	}

	customIntegrationTxAdClickLogicModel interface {
		// CountByWechatOpenId 按 openid 计数去重（server_v2 CountTxAdClickByOpenID 同款）。
		CountByWechatOpenId(ctx context.Context, wechatOpenId string) (int64, error)
	}
)

// NewIntegrationTxAdClickModel returns a model for the database table.
func NewIntegrationTxAdClickModel(conn *gorm.DB) IntegrationTxAdClickModel {
	return &customIntegrationTxAdClickModel{
		defaultIntegrationTxAdClickModel: newIntegrationTxAdClickModel(conn),
	}
}

// CountByWechatOpenId 按 openid 计数（uk 去重仲裁的先行检查）。
func (m *customIntegrationTxAdClickModel) CountByWechatOpenId(ctx context.Context, wechatOpenId string) (int64, error) {
	var total int64
	err := m.conn.WithContext(ctx).Model(&IntegrationTxAdClick{}).
		Where("`wechat_open_id` = ?", wechatOpenId).Count(&total).Error
	return total, err
}
