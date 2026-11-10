package gacha_play

import (
	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ DigBoxConfigsModel = (*customDigBoxConfigsModel)(nil)

type (
	// DigBoxConfigsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDigBoxConfigsModel.
	DigBoxConfigsModel interface {
		digBoxConfigsModel
		customDigBoxConfigsLogicModel
	}

	customDigBoxConfigsModel struct {
		*defaultDigBoxConfigsModel
	}

	customDigBoxConfigsLogicModel interface {
	}
)

// NewDigBoxConfigsModel returns a model for the database table.
func NewDigBoxConfigsModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) DigBoxConfigsModel {
	return &customDigBoxConfigsModel{
		defaultDigBoxConfigsModel: &defaultDigBoxConfigsModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`dig_box_configs`",
		},
	}
}

func (m *defaultDigBoxConfigsModel) customCacheKeys(data *DigBoxConfigs) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
