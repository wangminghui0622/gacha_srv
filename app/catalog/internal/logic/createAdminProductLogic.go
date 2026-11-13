package logic

import (
	"context"
	"fmt"
	"time"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
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
	if code == "" {
		code = fmt.Sprintf("PRD-%d", time.Now().UnixNano())
	}
	out, err := l.svcCtx.Repo.CreateAdminProduct(l.ctx, title, code, saleType, cover, in.IpId, in.BrandId, in.SeriesId, in.CostAmountMinor, in.SaleAmountMinor)
	if err != nil {
		return nil, mapAdminProductRepoErr(err)
	}
	return out, nil
}
