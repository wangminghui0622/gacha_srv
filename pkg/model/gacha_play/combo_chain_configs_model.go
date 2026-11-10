package gacha_play

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ ComboChainConfigsModel = (*customComboChainConfigsModel)(nil)

type (
	// ComboChainConfigsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customComboChainConfigsModel.
	ComboChainConfigsModel interface {
		comboChainConfigsModel
		customComboChainConfigsLogicModel
	}

	customComboChainConfigsModel struct {
		*defaultComboChainConfigsModel
	}

	customComboChainConfigsLogicModel interface {
	}
)

// NewComboChainConfigsModel returns a model for the database table.
func NewComboChainConfigsModel(conn *gorm.DB, c cache.CacheConf) ComboChainConfigsModel {
	return &customComboChainConfigsModel{
		defaultComboChainConfigsModel: newComboChainConfigsModel(conn, c),
	}
}

func (m *defaultComboChainConfigsModel) customCacheKeys(data *ComboChainConfigs) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
