package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateBrandLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateBrandLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateBrandLogic {
	return &UpdateBrandLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateBrand 编辑品牌名称/Logo。
func (l *UpdateBrandLogic) UpdateBrand(in *catalog.UpdateBrandRequest) (*catalog.Brand, error) {
	if in.BrandId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "品牌 ID 必填")
	}
	name, logo, err := core.BuildBrand(in.Name, in.Logo)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, err.Error())
	}
	out, err := l.svcCtx.Repo.UpdateBrand(l.ctx, in.BrandId, name, logo)
	if err != nil {
		return nil, mapBrandRepoErr(err)
	}
	return out, nil
}
