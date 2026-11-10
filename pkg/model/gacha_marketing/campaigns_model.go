package gacha_marketing

import (
	"context"

	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ CampaignsModel = (*customCampaignsModel)(nil)

type (
	// CampaignsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCampaignsModel.
	CampaignsModel interface {
		campaignsModel
		customCampaignsLogicModel
	}

	customCampaignsModel struct {
		*defaultCampaignsModel
	}

	customCampaignsLogicModel interface {
		// DeleteCache 手动失效缓存键（M14d 发布用）：发布走 repo raw UPDATE
		// 绕过模型 Update 的缓存失效，commit 后由调用方调用（best-effort，
		// 失败只记日志，兜底靠 30s 短 TTL）。
		DeleteCache(ctx context.Context, data *Campaigns) error
	}
)

// NewCampaignsModel returns a model for the database table.
func NewCampaignsModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) CampaignsModel {
	return &customCampaignsModel{
		defaultCampaignsModel: &defaultCampaignsModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`campaigns`",
		},
	}
}

func (m *defaultCampaignsModel) customCacheKeys(data *Campaigns) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}

// DeleteCache 见接口注释
func (m *customCampaignsModel) DeleteCache(ctx context.Context, data *Campaigns) error {
	return m.CachedConn.DelCacheCtx(ctx, m.GetCacheKeys(data)...)
}
