package gameimpl

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// GetPlayPoolVersion 查询统一玩赏池版本详情。
func GetPlayPoolVersion(ctx context.Context, svcCtx *svc.ServiceContext, in *play.GetPlayPoolVersionReq) (*play.GetPlayPoolVersionResp, error) {
	if in.PoolId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "poolId 非法")
	}
	var v gamemodelnc.PlayPoolVersion
	q := svcCtx.DB.Where("pool_id = ?", in.PoolId)
	if in.Version > 0 {
		q = q.Where("version = ?", in.Version)
	} else {
		q = q.Order("version DESC")
	}
	if err := q.First(&v).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.InvalidArgument, "赏池版本不存在")
		}
		return nil, err
	}
	var prizes []gamemodelnc.PlayPoolPrize
	if err := svcCtx.DB.Where("pool_id = ? AND version = ?", v.PoolId, v.Version).
		Order("prize_index ASC").Find(&prizes).Error; err != nil {
		return nil, err
	}
	publishedAt := int64(0)
	if v.PublishedAt != nil {
		publishedAt = v.PublishedAt.Unix()
	}
	return &play.GetPlayPoolVersionResp{
		PoolId:      v.PoolId,
		Version:     v.Version,
		Mode:        v.Mode,
		Name:        v.Name,
		PriceFen:    v.PriceFen,
		ConfigJson:  v.ConfigJson,
		ConfigHash:  v.ConfigHash,
		Status:      v.Status,
		PublishedAt: publishedAt,
		Prizes:      playPoolPrizesToProto(prizes),
	}, nil
}

// ListPlayPools 分页列出统一玩赏池（按 poolId 升序，返回每个池的最新版本摘要）。
func ListPlayPools(ctx context.Context, svcCtx *svc.ServiceContext, in *play.ListPlayPoolsReq) (*play.ListPlayPoolsResp, error) {
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	q := svcCtx.DB.Model(&gamemodelnc.PlayPoolHead{}).Where("id > ?", in.AfterPoolId)
	if in.Mode != "" {
		q = q.Where("mode = ?", in.Mode)
	}
	var heads []gamemodelnc.PlayPoolHead
	if err := q.Order("id ASC").Limit(int(pageSize)).Find(&heads).Error; err != nil {
		return nil, err
	}
	items := make([]*play.PlayPoolSummary, 0, len(heads))
	for _, h := range heads {
		var v gamemodelnc.PlayPoolVersion
		if err := svcCtx.DB.Where("pool_id = ?", h.Id).Order("version DESC").First(&v).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		publishedAt := int64(0)
		if v.PublishedAt != nil {
			publishedAt = v.PublishedAt.Unix()
		}
		items = append(items, &play.PlayPoolSummary{
			PoolId:        h.Id,
			LatestVersion: v.Version,
			Mode:          h.Mode,
			Name:          h.Name,
			Status:        v.Status,
			PublishedAt:   publishedAt,
		})
	}
	return &play.ListPlayPoolsResp{Items: items}, nil
}
