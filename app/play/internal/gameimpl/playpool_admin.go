package gameimpl

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gacha_srv/app/play/internal/playpool"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/pagination"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// ListAdminPlayPools 后台赏池列表（筛选 + 分页）。
func ListAdminPlayPools(ctx context.Context, svcCtx *svc.ServiceContext, in *play.ListAdminPlayPoolsReq) (*play.ListAdminPlayPoolsResp, error) {
	page, pageSize := pagination.Normalize(in.Page, in.PageSize, 15)
	query := func() *gorm.DB {
		q := svcCtx.DB.WithContext(ctx).Table("play_pool_catalog AS c")
		if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("(c.name LIKE ? OR CAST(c.id AS CHAR) LIKE ?)", like, like)
		}
		if mode := strings.TrimSpace(in.Mode); mode != "" {
			q = q.Where("c.mode = ?", mode)
		}
		if in.IpId != 0 {
			q = q.Where("c.ip_id = ?", in.IpId)
		}
		if tag := strings.TrimSpace(in.Tag); tag != "" {
			q = q.Where("c.tags_json LIKE ?", "%\""+tag+"\"%")
		}
		switch strings.TrimSpace(in.Status) {
		case "draft":
			q = q.Where("c.status = ?", playpool.StatusDraft)
		case "published", "live":
			q = q.Where("c.status = ?", playpool.StatusLive)
		case "offline":
			q = q.Where("c.status = ?", playpool.StatusOffline)
		case "retired", "void":
			q = q.Where("c.status = ?", playpool.StatusVoid)
		}
		return q
	}
	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, err
	}
	order := "c.created_at DESC, c.id DESC"
	if strings.EqualFold(in.SortOrder, "asc") {
		order = "c.created_at ASC, c.id ASC"
	}
	var rows []playpool.CatalogRow
	offset := int64(page-1) * int64(pageSize)
	if err := query().Select("c.*").Order(order).Offset(int(offset)).Limit(int(pageSize)).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*play.AdminPlayPool, 0, len(rows))
	for i := range rows {
		items = append(items, adminPoolToProtoV2(&rows[i]))
	}
	return &play.ListAdminPlayPoolsResp{Items: items, Total: uint64(total), Page: page, PageSize: pageSize}, nil
}

// UpdateAdminPlayPool 编辑赏池元数据与当前价格。
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
		var current playpool.CatalogRow
		if err := tx.Where("id = ?", in.PoolId).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.InvalidArgument, "赏池不存在")
			}
			return err
		}
		if err := tx.Model(&playpool.CatalogRow{}).Where("id = ?", in.PoolId).Updates(map[string]any{
			"name": name, "ip_id": in.IpId, "tags_json": tags, "weight": in.Weight,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		if in.PriceFen > 0 {
			if err := tx.Table("play_"+current.Mode+"_pools").Where("pool_id = ?", in.PoolId).
				Updates(map[string]any{"price_fen": in.PriceFen, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
			if err := tx.Model(&playpool.CatalogRow{}).Where("id = ?", in.PoolId).
				Update("price_fen", in.PriceFen).Error; err != nil {
				return err
			}
		}
		return playpool.WriteHistory(tx, in.PoolId, current.Mode, "UPDATE", "", "", "", 0)
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
			if err := tx.Model(&playpool.CatalogRow{}).Where("id = ?", w.PoolId).Update("weight", w.Weight).Error; err != nil {
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

// DeleteAdminPlayPools 删除赏池、当前配置和奖池；历史保留。
func DeleteAdminPlayPools(ctx context.Context, svcCtx *svc.ServiceContext, in *play.DeleteAdminPlayPoolsReq) (*play.DeleteAdminPlayPoolsResp, error) {
	if len(in.PoolIds) == 0 {
		return &play.DeleteAdminPlayPoolsResp{Deleted: 0}, nil
	}
	var deleted int64
	err := svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, poolID := range in.PoolIds {
			var row playpool.CatalogRow
			if err := tx.Where("id = ?", poolID).First(&row).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			if err := tx.Table("play_"+row.Mode+"_prizes").Where("pool_id = ?", poolID).Delete(&playpool.PrizeRow{}).Error; err != nil {
				return err
			}
			if err := tx.Table("play_"+row.Mode+"_pools").Where("pool_id = ?", poolID).Delete(&playpool.PoolRow{}).Error; err != nil {
				return err
			}
			if err := tx.Where("pool_id = ?", poolID).Delete(&playpool.RequestRow{}).Error; err != nil {
				return err
			}
			res := tx.Where("id = ?", poolID).Delete(&playpool.CatalogRow{})
			if res.Error != nil {
				return res.Error
			}
			deleted += res.RowsAffected
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &play.DeleteAdminPlayPoolsResp{Deleted: deleted}, nil
}

func getAdminPool(ctx context.Context, svcCtx *svc.ServiceContext, poolID int64) (*play.AdminPlayPool, error) {
	var row playpool.CatalogRow
	if err := svcCtx.DB.WithContext(ctx).Where("id = ?", poolID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.InvalidArgument, "赏池不存在")
		}
		return nil, err
	}
	return adminPoolToProtoV2(&row), nil
}

func adminPoolToProtoV2(row *playpool.CatalogRow) *play.AdminPlayPool {
	startAt := int64(0)
	if row.PublishedAt != nil {
		startAt = row.PublishedAt.Unix()
	}
	return &play.AdminPlayPool{
		PoolId:        row.ID,
		Name:          row.Name,
		Mode:          row.Mode,
		IpId:          uint64(row.IPID),
		Tags:          decodePoolTags(row.TagsJSON),
		Periods:       0,
		Weight:        int64(row.Weight),
		LatestVersion: 1,
		PriceFen:      row.PriceFen,
		Status:        legacyPoolStatus(row.Status),
		StartAt:       startAt,
		CreatedAt:     row.CreatedAt.Unix(),
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
