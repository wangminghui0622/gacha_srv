package gacha_play

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ MemoryMatchEntriesModel = (*customMemoryMatchEntriesModel)(nil)

type (
	// MemoryMatchEntriesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMemoryMatchEntriesModel.
	MemoryMatchEntriesModel interface {
		memoryMatchEntriesModel
		customMemoryMatchEntriesLogicModel
	}

	customMemoryMatchEntriesModel struct {
		*defaultMemoryMatchEntriesModel
	}

	customMemoryMatchEntriesLogicModel interface {
	}
)

// NewMemoryMatchEntriesModel returns a model for the database table.
func NewMemoryMatchEntriesModel(conn *gorm.DB, c cache.CacheConf) MemoryMatchEntriesModel {
	return &customMemoryMatchEntriesModel{
		defaultMemoryMatchEntriesModel: newMemoryMatchEntriesModel(conn, c),
	}
}

func (m *defaultMemoryMatchEntriesModel) customCacheKeys(data *MemoryMatchEntries) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
