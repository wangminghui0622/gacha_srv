package gacha_play

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ GroupBuyConfigsModel = (*customGroupBuyConfigsModel)(nil)

type (
	// GroupBuyConfigsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGroupBuyConfigsModel.
	GroupBuyConfigsModel interface {
		groupBuyConfigsModel
		customGroupBuyConfigsLogicModel
	}

	customGroupBuyConfigsModel struct {
		*defaultGroupBuyConfigsModel
	}

	customGroupBuyConfigsLogicModel interface {
	}
)

// NewGroupBuyConfigsModel returns a model for the database table.
func NewGroupBuyConfigsModel(conn *gorm.DB, c cache.CacheConf) GroupBuyConfigsModel {
	return &customGroupBuyConfigsModel{
		defaultGroupBuyConfigsModel: newGroupBuyConfigsModel(conn, c),
	}
}

func (m *defaultGroupBuyConfigsModel) customCacheKeys(data *GroupBuyConfigs) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
