package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAdminProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAdminProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAdminProductLogic {
	return &UpdateAdminProductLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// UpdateAdminProduct 编辑后台商品展示字段。
func (l *UpdateAdminProductLogic) UpdateAdminProduct(in *catalog.UpdateAdminProductRequest) (*catalog.AdminProduct, error) {
	if in.ProductId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品 ID 必填")
	}
	title, _, saleType, cover, err := core.BuildAdminProduct(in.Title, "", in.SaleType, in.CoverUrl, in.CostAmountMinor, in.SaleAmountMinor)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, err.Error())
	}
	out, err := l.svcCtx.Repo.UpdateAdminProduct(l.ctx, in.ProductId, title, in.IpId, in.BrandId, in.SeriesId, saleType, cover, in.CostAmountMinor, in.SaleAmountMinor)
	if err != nil {
		return nil, mapAdminProductRepoErr(err)
	}
	return out, nil
}
