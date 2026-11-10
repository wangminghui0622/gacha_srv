package gacha_play

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ MemoryMatchConfigsModel = (*customMemoryMatchConfigsModel)(nil)

type (
	// MemoryMatchConfigsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchConfigsModel.
	MemoryMatchConfigsModel interface {
		memoryMatchConfigsModel
		customMemoryMatchConfigsLogicModel
	}

	customMemoryMatchConfigsModel struct {
		*defaultMemoryMatchConfigsModel
	}

	customMemoryMatchConfigsLogicModel interface {
	}
)

// NewMemoryMatchConfigsModel returns a model for the database table.
func NewMemoryMatchConfigsModel(conn *gorm.DB, c cache.CacheConf) MemoryMatchConfigsModel {
	return &customMemoryMatchConfigsModel{
		defaultMemoryMatchConfigsModel: newMemoryMatchConfigsModel(conn, c),
	}
}

func (m *defaultMemoryMatchConfigsModel) customCacheKeys(data *MemoryMatchConfigs) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
