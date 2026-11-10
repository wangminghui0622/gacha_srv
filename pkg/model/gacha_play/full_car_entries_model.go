package gacha_play

import (
	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ FullCarEntriesModel = (*customFullCarEntriesModel)(nil)

type (
	// FullCarEntriesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customFullCarEntriesModel.
	FullCarEntriesModel interface {
		fullCarEntriesModel
		customFullCarEntriesLogicModel
	}

	customFullCarEntriesModel struct {
		*defaultFullCarEntriesModel
	}

	customFullCarEntriesLogicModel interface {
	}
)

// NewFullCarEntriesModel returns a model for the database table.
func NewFullCarEntriesModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) FullCarEntriesModel {
	return &customFullCarEntriesModel{
		defaultFullCarEntriesModel: &defaultFullCarEntriesModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`full_car_entries`",
		},
	}
}

func (m *defaultFullCarEntriesModel) customCacheKeys(data *FullCarEntries) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
