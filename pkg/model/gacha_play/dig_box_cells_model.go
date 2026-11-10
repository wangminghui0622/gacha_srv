package gacha_play

import (
	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ DigBoxCellsModel = (*customDigBoxCellsModel)(nil)

type (
	// DigBoxCellsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDigBoxCellsModel.
	DigBoxCellsModel interface {
		digBoxCellsModel
		customDigBoxCellsLogicModel
	}

	customDigBoxCellsModel struct {
		*defaultDigBoxCellsModel
	}

	customDigBoxCellsLogicModel interface {
	}
)

// NewDigBoxCellsModel returns a model for the database table.
func NewDigBoxCellsModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) DigBoxCellsModel {
	return &customDigBoxCellsModel{
		defaultDigBoxCellsModel: &defaultDigBoxCellsModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`dig_box_cells`",
		},
	}
}

func (m *defaultDigBoxCellsModel) customCacheKeys(data *DigBoxCells) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
