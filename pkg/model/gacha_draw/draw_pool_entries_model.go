package gacha_draw

import (
	"context"
	"fmt"

	"github.com/SpectatorNan/gorm-zero/gormc"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"
)

// entriesBoxListKeyPrefix 奖池条目「按宝箱列表」缓存键前缀（BoxDetail 热路径读）。
// 键格式与生成代码的主键键一致：cache:<域>:<表>:box:<boxId>
const entriesBoxListKeyPrefix = "cache:gachaDraw:drawPoolEntries:box:"

var _ DrawPoolEntriesModel = (*customDrawPoolEntriesModel)(nil)

type (
	// DrawPoolEntriesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDrawPoolEntriesModel.
	DrawPoolEntriesModel interface {
		drawPoolEntriesModel
		customDrawPoolEntriesLogicModel
	}

	customDrawPoolEntriesModel struct {
		*defaultDrawPoolEntriesModel
	}

	customDrawPoolEntriesLogicModel interface {
		// UpdateByKeys 与生成的 Update 等价，但去掉「先 FindOne 拿旧键」这步：
		// 写路径都在持 FOR UPDATE 行锁时调用，而 FindOne 走模型自己的连接池——
		// 缓存未命中时要在持锁状态下再取第二条连接，高并发写会把连接池耗尽。
		// 额外失效 box 列表键：BoxDetail 的列表缓存读也要跟着条目变化失效。
		UpdateByKeys(ctx context.Context, tx *gorm.DB, data *DrawPoolEntries) error
		// DeleteCache 手动失效缓存键（延迟双删用）：
		// 写事务 commit 成功后由调用方延迟调用，清掉「DEL 后 commit 前」窗口期
		// 被并发读回填的旧值。复用 GetCacheKeys 算键，key 格式不会漂移。
		DeleteCache(ctx context.Context, data *DrawPoolEntries) error
		// FindAllByBoxIdCached 缓存读宝箱全部条目（BoxDetail 概率公示热路径）：
		// cache-aside，未命中查库（id 升序）后回填；key 随写路径一起失效。
		FindAllByBoxIdCached(ctx context.Context, boxId int64) ([]DrawPoolEntries, error)
		// ReplaceByBoxId 发布物化：按宝箱整体替换条目（查旧行 → 删旧行 → 批量插新行，
		// 一次 ExecCtx 完成，失效旧行主键键 + box 列表键）。tx 由发布事务传入。
		ReplaceByBoxId(ctx context.Context, tx *gorm.DB, boxId int64, news []DrawPoolEntries) error
	}
)

// NewDrawPoolEntriesModel returns a model for the database table.
// opts 透传给缓存节点：remaining 每次开奖变化 → 短 TTL 3 分钟兜底
// （配 commit 后延迟双删，见 SettleDraw 写路径）。
// 这里不调用生成的 newDrawPoolEntriesModel（它不接受 opts），构造体与其保持一致。
func NewDrawPoolEntriesModel(conn *gorm.DB, c cache.CacheConf, opts ...cache.Option) DrawPoolEntriesModel {
	return &customDrawPoolEntriesModel{
		defaultDrawPoolEntriesModel: &defaultDrawPoolEntriesModel{
			CachedConn: gormc.NewConn(conn, c, opts...),
			table:      "`draw_pool_entries`",
		},
	}
}

func (m *defaultDrawPoolEntriesModel) customCacheKeys(data *DrawPoolEntries) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}

// UpdateByKeys 见接口注释：SQL(Save, 走 tx) → 删主键键 + box 列表键，无 FindOne。
// 顺序铁律：先 MySQL（未提交）→ 后删缓存 → 最后 commit；
// 缓存删除失败不会回滚事务（go-zero 异步重试自愈），一致性靠延迟双删 + 短 TTL。
func (m *customDrawPoolEntriesModel) UpdateByKeys(ctx context.Context, tx *gorm.DB, data *DrawPoolEntries) error {
	keys := append(m.GetCacheKeys(data), m.boxListKey(data.BoxId))
	return m.CachedConn.ExecCtx(ctx, func(conn *gorm.DB) error {
		db := conn
		if tx != nil {
			db = tx
		}
		return db.Save(data).Error
	}, keys...)
}

// DeleteCache 见接口注释：commit 后延迟双删（best-effort，失败只记日志，兜底靠短 TTL）
func (m *customDrawPoolEntriesModel) DeleteCache(ctx context.Context, data *DrawPoolEntries) error {
	keys := append(m.GetCacheKeys(data), m.boxListKey(data.BoxId))
	return m.CachedConn.DelCacheCtx(ctx, keys...)
}

// boxListKey 条目列表缓存键
func (m *customDrawPoolEntriesModel) boxListKey(boxId int64) string {
	return fmt.Sprintf("%s%d", entriesBoxListKeyPrefix, boxId)
}

// ReplaceByBoxId 见接口注释：先查旧行拿主键键（删后取不到 id），再删旧行、批量插新行；
// 缓存键（旧行主键键 + box 列表键）在闭包执行后统一失效，事务一致性由调用方 commit 保证。
func (m *customDrawPoolEntriesModel) ReplaceByBoxId(ctx context.Context, tx *gorm.DB, boxId int64, news []DrawPoolEntries) error {
	keys := []string{m.boxListKey(boxId)}
	return m.CachedConn.ExecCtx(ctx, func(conn *gorm.DB) error {
		db := conn
		if tx != nil {
			db = tx
		}
		var olds []DrawPoolEntries
		if err := db.Where("`box_id` = ?", boxId).Find(&olds).Error; err != nil {
			return err
		}
		for i := range olds {
			keys = append(keys, m.GetCacheKeys(&olds[i])...)
		}
		if err := db.Where("`box_id` = ?", boxId).Delete(&DrawPoolEntries{}).Error; err != nil {
			return err
		}
		if len(news) == 0 {
			return nil
		}
		return db.Create(&news).Error
	}, keys...)
}

// FindAllByBoxIdCached 见接口注释
func (m *customDrawPoolEntriesModel) FindAllByBoxIdCached(ctx context.Context, boxId int64) ([]DrawPoolEntries, error) {
	var entries []DrawPoolEntries
	err := m.QueryCtx(ctx, &entries, m.boxListKey(boxId), func(conn *gorm.DB) error {
		return conn.Where("`box_id` = ?", boxId).Order("`id` ASC").Find(&entries).Error
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
