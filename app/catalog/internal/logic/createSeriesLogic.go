package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateSeriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSeriesLogic {
	return &CreateSeriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateSeries 创建系列。
func (l *CreateSeriesLogic) CreateSeries(in *catalog.CreateSeriesRequest) (*catalog.Series, error) {
	name, brandID, sortWeight, err := core.BuildSeries(in.Name, in.BrandId, in.SortWeight)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, err.Error())
	}
	out, err := l.svcCtx.Repo.CreateSeries(l.ctx, name, brandID, sortWeight)
	if err != nil {
		return nil, mapSeriesRepoErr(err)
	}
	return out, nil
}
