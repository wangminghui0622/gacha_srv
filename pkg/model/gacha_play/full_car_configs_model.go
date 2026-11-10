package gacha_play

import (
	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ FullCarConfigsModel = (*customFullCarConfigsModel)(nil)

type (
	// FullCarConfigsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customFullCarConfigsModel.
	FullCarConfigsModel interface {
		fullCarConfigsModel
		customFullCarConfigsLogicModel
	}

	customFullCarConfigsModel struct {
		*defaultFullCarConfigsModel
	}

	customFullCarConfigsLogicModel interface {
	}
)

// NewFullCarConfigsModel returns a model for the database table.
func NewFullCarConfigsModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) FullCarConfigsModel {
	return &customFullCarConfigsModel{
		defaultFullCarConfigsModel: &defaultFullCarConfigsModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`full_car_configs`",
		},
	}
}

func (m *defaultFullCarConfigsModel) customCacheKeys(data *FullCarConfigs) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
