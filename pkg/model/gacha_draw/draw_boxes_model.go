package gacha_draw

import (
	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ DrawBoxesModel = (*customDrawBoxesModel)(nil)

type (
	// DrawBoxesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDrawBoxesModel.
	DrawBoxesModel interface {
		drawBoxesModel
		customDrawBoxesLogicModel
	}

	customDrawBoxesModel struct {
		*defaultDrawBoxesModel
	}

	customDrawBoxesLogicModel interface {
	}
)

// NewDrawBoxesModel returns a model for the database table.
// opts 透传给缓存节点：宝箱是静态配置，v1 无写路径；给 10 分钟 TTL 兜底，
// 未来若出现管理端改价/下架旁路直写 DB，缓存最多陈旧 10 分钟自愈。
// 这里不调用生成的 newDrawBoxesModel（它不接受 opts），构造体与其保持一致。
func NewDrawBoxesModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) DrawBoxesModel {
	return &customDrawBoxesModel{
		defaultDrawBoxesModel: &defaultDrawBoxesModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`draw_boxes`",
		},
	}
}

func (m *defaultDrawBoxesModel) customCacheKeys(data *DrawBoxes) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
