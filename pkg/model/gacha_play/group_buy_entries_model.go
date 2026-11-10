package gacha_play

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ GroupBuyEntriesModel = (*customGroupBuyEntriesModel)(nil)

type (
	// GroupBuyEntriesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGroupBuyEntriesModel.
	GroupBuyEntriesModel interface {
		groupBuyEntriesModel
		customGroupBuyEntriesLogicModel
	}

	customGroupBuyEntriesModel struct {
		*defaultGroupBuyEntriesModel
	}

	customGroupBuyEntriesLogicModel interface {
	}
)

// NewGroupBuyEntriesModel returns a model for the database table.
func NewGroupBuyEntriesModel(conn *gorm.DB, c cache.CacheConf) GroupBuyEntriesModel {
	return &customGroupBuyEntriesModel{
		defaultGroupBuyEntriesModel: newGroupBuyEntriesModel(conn, c),
	}
}

func (m *defaultGroupBuyEntriesModel) customCacheKeys(data *GroupBuyEntries) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
