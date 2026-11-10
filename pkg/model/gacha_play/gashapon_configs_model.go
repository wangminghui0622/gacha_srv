package gacha_play

import (
	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ GashaponConfigsModel = (*customGashaponConfigsModel)(nil)

type (
	// GashaponConfigsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGashaponConfigsModel.
	GashaponConfigsModel interface {
		gashaponConfigsModel
		customGashaponConfigsLogicModel
	}

	customGashaponConfigsModel struct {
		*defaultGashaponConfigsModel
	}

	customGashaponConfigsLogicModel interface {
	}
)

// NewGashaponConfigsModel returns a model for the database table.
func NewGashaponConfigsModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) GashaponConfigsModel {
	return &customGashaponConfigsModel{
		defaultGashaponConfigsModel: &defaultGashaponConfigsModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`gashapon_configs`",
		},
	}
}

func (m *defaultGashaponConfigsModel) customCacheKeys(data *GashaponConfigs) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
