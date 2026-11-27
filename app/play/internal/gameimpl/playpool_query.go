package gameimpl

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/playpool"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// GetPlayPoolVersion 查询赏池当前配置。旧接口名保留；version 固定为 1。
func GetPlayPoolVersion(ctx context.Context, svcCtx *svc.ServiceContext, in *play.GetPlayPoolVersionReq) (*play.GetPlayPoolVersionResp, error) {
	if in.PoolId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "poolId 非法")
	}
	p, err := playpool.Get(svcCtx.DB, in.PoolId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.InvalidArgument, "赏池不存在")
		}
		return nil, err
	}
	publishedAt := int64(0)
	if p.PublishedAt != nil {
		publishedAt = p.PublishedAt.Unix()
	}
	return &play.GetPlayPoolVersionResp{
		PoolId:      p.ID,
		Version:     1,
		Mode:        p.Mode,
		Name:        p.Name,
		PriceFen:    p.PriceFen,
		ConfigJson:  p.ConfigJSON,
		ConfigHash:  "",
		Status:      legacyPoolStatus(p.Status),
		PublishedAt: publishedAt,
		Prizes:      playPoolPrizesToProtoV2(p),
		AdminJson:   p.AdminJSON,
	}, nil
}

// ListPlayPools 分页列出赏池；每个池固定 version=1。
func ListPlayPools(ctx context.Context, svcCtx *svc.ServiceContext, in *play.ListPlayPoolsReq) (*play.ListPlayPoolsResp, error) {
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	q := svcCtx.DB.WithContext(ctx).Model(&playpool.CatalogRow{}).Where("id > ?", in.AfterPoolId)
	if in.Mode != "" {
		q = q.Where("mode = ?", in.Mode)
	}
	var rows []playpool.CatalogRow
	if err := q.Order("id ASC").Limit(int(pageSize)).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*play.PlayPoolSummary, 0, len(rows))
	for _, row := range rows {
		publishedAt := int64(0)
		if row.PublishedAt != nil {
			publishedAt = row.PublishedAt.Unix()
		}
		items = append(items, &play.PlayPoolSummary{
			PoolId:        row.ID,
			LatestVersion: 1,
			Mode:          row.Mode,
			Name:          row.Name,
			Status:        legacyPoolStatus(row.Status),
			PublishedAt:   publishedAt,
		})
	}
	return &play.ListPlayPoolsResp{Items: items}, nil
}

func legacyPoolStatus(status int64) int64 {
	switch status {
	case playpool.StatusDraft:
		return 1
	case playpool.StatusLive:
		return 2
	case playpool.StatusOffline, playpool.StatusVoid:
		return 3
	default:
		return status
	}
}

func playPoolPrizesToProtoV2(p *playpool.Pool) []*play.PlayPoolPrize {
	out := make([]*play.PlayPoolPrize, 0, len(p.Prizes))
	for _, prize := range p.Prizes {
		out = append(out, &play.PlayPoolPrize{
			PrizeIndex:       int64(prize.Index),
			PrizeGrade:       prize.PrizeGrade,
			SpecialKind:      prize.SpecialKind,
			PrizeType:        int64(prize.PrizeType),
			Name:             prize.Name,
			SkuId:            prize.SkuID,
			ChildPoolId:      prize.ChildPoolID,
			Weight:           prize.Weight,
			Quantity:         int64(prize.Quantity),
			Remaining:        int64(prize.Quantity),
			IsPrize:          prize.IsPrize,
			TargetValue:      int64(prize.TargetValue),
			CashFen:          prize.CashFen,
			PointsAmount:     prize.PointsAmount,
			CouponTemplateNo: prize.CouponTemplateNo,
			CostFen:          prize.CostFen,
			ImageUrl:         prize.ImageURL,
			ParentIndex:      int64(prize.ParentIndex),
			ExtraJson:        prize.ExtraJSON,
			HasParent:        prize.ParentIndex >= 0,
		})
	}
	return out
}
