package logic

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/repo"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBoxPoolsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBoxPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBoxPoolsLogic {
	return &ListBoxPoolsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListBoxPools 无限赏宝箱列表：每个赏池仅返回最新版本，含后台展示字段。
func (l *ListBoxPoolsLogic) ListBoxPools(in *draw.ListBoxPoolsReq) (*draw.ListBoxPoolsResp, error) {
	page := int(in.GetPage())
	if page <= 0 {
		page = 1
	}
	pageSize := clampPageSize(in.GetPageSize())
	rows, total, err := repo.ListLatestPoolVersions(
		l.svcCtx.DB, in.GetKeyword(), (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}

	resp := &draw.ListBoxPoolsResp{Total: total}
	for i := range rows {
		box, err := poolDecode(&rows[i])
		if err != nil {
			return nil, err
		}
		resp.Items = append(resp.Items, &draw.BoxPoolSummary{
			PoolId:                box.PoolId,
			Version:               box.Version,
			Status:                box.Status,
			BoxName:               box.BoxName,
			Price:                 box.Price,
			Periods:               box.Periods,
			StartAt:               box.StartAt,
			EndAt:                 box.EndAt,
			Discount:              box.Discount,
			SortWeight:            box.SortWeight,
			CoverImage:            box.CoverImage,
			CoverThumb:            box.CoverThumb,
			ConfigHash:            box.ConfigHash,
			PublishedAtUnixMillis: box.PublishedAtUnixMillis,
			UpdatedAtUnixMillis:   rows[i].CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
