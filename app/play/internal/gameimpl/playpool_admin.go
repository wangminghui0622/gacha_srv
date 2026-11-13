package gameimpl

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/pagination"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type adminPoolRow struct {
	ID            int64     `gorm:"column:id"`
	Name          string    `gorm:"column:name"`
	Mode          string    `gorm:"column:mode"`
	IPID          uint64    `gorm:"column:ip_id"`
	TagsJSON      string    `gorm:"column:tags_json"`
	Periods       int64     `gorm:"column:periods"`
	Weight        int64     `gorm:"column:weight"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	LatestVersion int64     `gorm:"column:latest_version"`
	PriceFen      int64     `gorm:"column:price_fen"`
	Status        int64     `gorm:"column:status"`
	StartAt       int64     `gorm:"column:start_at"`
}

const adminPoolSelect = "h.id, h.name, h.mode, h.ip_id, h.tags_json, h.periods, h.weight, h.created_at, " +
	"COALESCE((SELECT v.version FROM play_pool_versions AS v WHERE v.pool_id = h.id ORDER BY v.version DESC LIMIT 1), 0) AS latest_version, " +
	"COALESCE((SELECT v.price_fen FROM play_pool_versions AS v WHERE v.pool_id = h.id ORDER BY v.version DESC LIMIT 1), 0) AS price_fen, " +
	"COALESCE((SELECT v.status FROM play_pool_versions AS v WHERE v.pool_id = h.id ORDER BY v.version DESC LIMIT 1), 0) AS status, " +
	"COALESCE((SELECT UNIX_TIMESTAMP(v.published_at) FROM play_pool_versions AS v WHERE v.pool_id = h.id ORDER BY v.version DESC LIMIT 1), 0) AS start_at"

// ListAdminPlayPools 后台赏池列表（筛选 + 分页）。
func ListAdminPlayPools(ctx context.Context, svcCtx *svc.ServiceContext, in *play.ListAdminPlayPoolsReq) (*play.ListAdminPlayPoolsResp, error) {
	page, pageSize := pagination.Normalize(in.Page, in.PageSize, 15)
	query := func() *gorm.DB {
		q := svcCtx.DB.WithContext(ctx).Table("play_pool_heads AS h")
		if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("(h.name LIKE ? OR CAST(h.id AS CHAR) LIKE ?)", like, like)
		}
		if strings.TrimSpace(in.Mode) != "" {
			q = q.Where("h.mode = ?", strings.TrimSpace(in.Mode))
		}
		if in.IpId != 0 {
			q = q.Where("h.ip_id = ?", in.IpId)
		}
		if tag := strings.TrimSpace(in.Tag); tag != "" {
			q = q.Where("h.tags_json LIKE ?", "%\""+tag+"\"%")
		}
		latestStatus := "(SELECT v.status FROM play_pool_versions AS v WHERE v.pool_id = h.id ORDER BY v.version DESC LIMIT 1)"
		switch strings.TrimSpace(in.Status) {
		case "draft":
			q = q.Where(latestStatus + " = 1")
		case "published":
			q = q.Where(latestStatus + " = 2")
		case "retired":
			q = q.Where(latestStatus + " = 3")
		}
		return q
	}
	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, err
	}
	order := "h.created_at DESC, h.id DESC"
	if strings.EqualFold(in.SortOrder, "asc") {
		order = "h.created_at ASC, h.id ASC"
	}
	var rows []adminPoolRow
	offset := int64(page-1) * int64(pageSize)
	if err := query().Select(adminPoolSelect).Order(order).Offset(int(offset)).Limit(int(pageSize)).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*play.AdminPlayPool, 0, len(rows))
	for _, row := range rows {
		items = append(items, adminPoolToProto(row))
	}
	return &play.ListAdminPlayPoolsResp{Items: items, Total: uint64(total), Page: page, PageSize: pageSize}, nil
}

// UpdateAdminPlayPool 编辑赏池元数据与最新版本价格。
func UpdateAdminPlayPool(ctx context.Context, svcCtx *svc.ServiceContext, in *play.UpdateAdminPlayPoolReq) (*play.AdminPlayPool, error) {
	if in.PoolId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "poolId 非法")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "赏池名称必填")
	}
	tags, err := encodePoolTags(in.Tags)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "标签格式错误")
	}
	err = svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var head struct{ ID int64 }
		if err := tx.Table("play_pool_heads").Select("id").Where("id = ?", in.PoolId).Take(&head).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.InvalidArgument, "赏池不存在")
			}
			return err
		}
		if err := tx.Table("play_pool_heads").Where("id = ?", in.PoolId).Updates(map[string]any{
			"name": name, "ip_id": in.IpId, "tags_json": tags,
			"periods": in.Periods, "weight": in.Weight,
		}).Error; err != nil {
			return err
		}
		if in.PriceFen > 0 {
			var version struct{ Version int64 }
			if err := tx.Table("play_pool_versions").Select("version").
				Where("pool_id = ?", in.PoolId).Order("version DESC").Take(&version).Error; err == nil {
				if err := tx.Table("play_pool_versions").Where("pool_id = ? AND version = ?", in.PoolId, version.Version).
					Update("price_fen", in.PriceFen).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return getAdminPool(ctx, svcCtx, in.PoolId)
}

// SaveAdminPlayPoolWeights 批量保存权重。
func SaveAdminPlayPoolWeights(ctx context.Context, svcCtx *svc.ServiceContext, in *play.SaveAdminPlayPoolWeightsReq) (*play.SaveAdminPlayPoolWeightsResp, error) {
	if len(in.Weights) == 0 {
		return &play.SaveAdminPlayPoolWeightsResp{Saved: true}, nil
	}
	err := svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, w := range in.Weights {
			if w.PoolId <= 0 {
				continue
			}
			if err := tx.Table("play_pool_heads").Where("id = ?", w.PoolId).Update("weight", w.Weight).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &play.SaveAdminPlayPoolWeightsResp{Saved: true}, nil
}

// DeleteAdminPlayPools 删除赏池及其版本/奖品/草稿幂等记录。
func DeleteAdminPlayPools(ctx context.Context, svcCtx *svc.ServiceContext, in *play.DeleteAdminPlayPoolsReq) (*play.DeleteAdminPlayPoolsResp, error) {
	if len(in.PoolIds) == 0 {
		return &play.DeleteAdminPlayPoolsResp{Deleted: 0}, nil
	}
	var deleted int64
	err := svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("pool_id IN ?", in.PoolIds).Delete(&gamemodelnc.PlayPoolPrize{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pool_id IN ?", in.PoolIds).Delete(&gamemodelnc.PlayPoolVersion{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pool_id IN ?", in.PoolIds).Delete(&gamemodelnc.PlayPoolDraft{}).Error; err != nil {
			return err
		}
		res := tx.Where("id IN ?", in.PoolIds).Delete(&gamemodelnc.PlayPoolHead{})
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &play.DeleteAdminPlayPoolsResp{Deleted: deleted}, nil
}

func getAdminPool(ctx context.Context, svcCtx *svc.ServiceContext, poolID int64) (*play.AdminPlayPool, error) {
	var row adminPoolRow
	if err := svcCtx.DB.WithContext(ctx).Table("play_pool_heads AS h").Select(adminPoolSelect).
		Where("h.id = ?", poolID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.InvalidArgument, "赏池不存在")
		}
		return nil, err
	}
	return adminPoolToProto(row), nil
}

func adminPoolToProto(row adminPoolRow) *play.AdminPlayPool {
	return &play.AdminPlayPool{
		PoolId: row.ID, Name: row.Name, Mode: row.Mode, IpId: row.IPID,
		Tags: decodePoolTags(row.TagsJSON), Periods: row.Periods, Weight: row.Weight,
		LatestVersion: row.LatestVersion, PriceFen: row.PriceFen, Status: row.Status,
		StartAt: row.StartAt, CreatedAt: row.CreatedAt.Unix(),
	}
}

func encodePoolTags(tags []string) (string, error) {
	clean := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		clean = append(clean, tag)
	}
	blob, err := json.Marshal(clean)
	return string(blob), err
}

func decodePoolTags(raw string) []string {
	var tags []string
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	if err := json.Unmarshal([]byte(raw), &tags); err != nil || tags == nil {
		return []string{}
	}
	return tags
}
