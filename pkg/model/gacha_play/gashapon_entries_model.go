package gacha_play

import (
	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ GashaponEntriesModel = (*customGashaponEntriesModel)(nil)

type (
	// GashaponEntriesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGashaponEntriesModel.
	GashaponEntriesModel interface {
		gashaponEntriesModel
		customGashaponEntriesLogicModel
	}

	customGashaponEntriesModel struct {
		*defaultGashaponEntriesModel
	}

	customGashaponEntriesLogicModel interface {
	}
)

// NewGashaponEntriesModel returns a model for the database table.
func NewGashaponEntriesModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) GashaponEntriesModel {
	return &customGashaponEntriesModel{
		defaultGashaponEntriesModel: &defaultGashaponEntriesModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`gashapon_entries`",
		},
	}
}

func (m *defaultGashaponEntriesModel) customCacheKeys(data *GashaponEntries) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
