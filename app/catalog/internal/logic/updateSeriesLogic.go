package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSeriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSeriesLogic {
	return &UpdateSeriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateSeries 编辑系列名称/品牌/排序权重。
func (l *UpdateSeriesLogic) UpdateSeries(in *catalog.UpdateSeriesRequest) (*catalog.Series, error) {
	if in.SeriesId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "系列 ID 必填")
	}
	name, brandID, sortWeight, err := core.BuildSeries(in.Name, in.BrandId, in.SortWeight)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, err.Error())
	}
	out, err := l.svcCtx.Repo.UpdateSeries(l.ctx, in.SeriesId, name, brandID, sortWeight)
	if err != nil {
		return nil, mapSeriesRepoErr(err)
	}
	return out, nil
}
