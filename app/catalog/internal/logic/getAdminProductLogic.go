package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAdminProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAdminProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAdminProductLogic {
	return &GetAdminProductLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetAdminProduct 读取后台商品编辑表单完整字段。
func (l *GetAdminProductLogic) GetAdminProduct(in *catalog.GetAdminProductRequest) (*catalog.AdminProductDetail, error) {
	if in.ProductId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品 ID 必填")
	}
	out, err := l.svcCtx.Repo.GetAdminProduct(l.ctx, in.ProductId)
	if err != nil {
		return nil, mapAdminProductRepoErr(err)
	}
	return out, nil
}
