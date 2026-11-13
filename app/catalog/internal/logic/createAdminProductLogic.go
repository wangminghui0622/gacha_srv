package logic

import (
	"context"
	"fmt"
	"time"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/repo"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAdminProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateAdminProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAdminProductLogic {
	return &CreateAdminProductLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateAdminProduct 创建后台商品。
func (l *CreateAdminProductLogic) CreateAdminProduct(in *catalog.CreateAdminProductRequest) (*catalog.AdminProduct, error) {
	title, code, saleType, cover, err := core.BuildAdminProduct(in.Title, in.ProductCode, in.SaleType, in.CoverUrl, in.CostAmountMinor, in.SaleAmountMinor)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, err.Error())
	}
	if in.IpId == 0 || in.BrandId == 0 || in.SeriesId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "IP、厂商、系列必填")
	}
	if code == "" {
		code = fmt.Sprintf("PRD-%d", time.Now().UnixNano())
	}
	saleAmount := in.SaleAmountMinor
	if saleAmount == 0 {
		saleAmount = in.CostAmountMinor
	}
	adminJSON, err := encodeAdminJSON(in.TotalStock, in.FrozenStock, in.HalfBodyImage, in.ThumbUrl, in.DetailIntro, in.DetailImages, in.DetailParams)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品扩展字段格式错误")
	}
	out, err := l.svcCtx.Repo.CreateAdminProduct(l.ctx, repo.AdminProductFields{
		Title: title, ProductCode: code, SaleType: saleType, CoverURL: cover, AdminJSON: adminJSON,
		IPID: in.IpId, BrandID: in.BrandId, SeriesID: in.SeriesId,
		CostAmountMinor: in.CostAmountMinor, SaleAmountMinor: saleAmount,
	})
	if err != nil {
		return nil, mapAdminProductRepoErr(err)
	}
	return out, nil
}
