package gacha_marketing

import (
	"context"

	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

var _ CouponTemplatesModel = (*customCouponTemplatesModel)(nil)

type (
	// CouponTemplatesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCouponTemplatesModel.
	CouponTemplatesModel interface {
		couponTemplatesModel
		customCouponTemplatesLogicModel
	}

	customCouponTemplatesModel struct {
		*defaultCouponTemplatesModel
	}

	customCouponTemplatesLogicModel interface {
		// DeleteCache 手动失效缓存键（M14d 发布物化用）：控制台发布走 raw upsert
		// 绕过模型 Update 的缓存失效，commit 后由调用方调用（best-effort，
		// 失败只记日志，兜底靠 30s 短 TTL）。
		DeleteCache(ctx context.Context, data *CouponTemplates) error
	}
)

// NewCouponTemplatesModel returns a model for the database table.
// opts 透传给缓存节点：券模板发布后改动低频，给 30 秒 TTL 兜底（与 base 域策略模型同款）。
// 这里不调用生成的 newCouponTemplatesModel（它不接受 opts），构造体与其保持一致。
func NewCouponTemplatesModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) CouponTemplatesModel {
	return &customCouponTemplatesModel{
		defaultCouponTemplatesModel: &defaultCouponTemplatesModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`coupon_templates`",
		},
	}
}

func (m *defaultCouponTemplatesModel) customCacheKeys(data *CouponTemplates) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}

// DeleteCache 见接口注释
func (m *customCouponTemplatesModel) DeleteCache(ctx context.Context, data *CouponTemplates) error {
	return m.CachedConn.DelCacheCtx(ctx, m.GetCacheKeys(data)...)
}

