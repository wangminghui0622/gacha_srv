package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CopyAdminProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCopyAdminProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CopyAdminProductLogic {
	return &CopyAdminProductLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CopyAdminProduct 复制商品。
func (l *CopyAdminProductLogic) CopyAdminProduct(in *catalog.CopyAdminProductRequest) (*catalog.AdminProduct, error) {
	if in.ProductId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品 ID 必填")
	}
	out, err := l.svcCtx.Repo.CopyAdminProduct(l.ctx, in.ProductId)
	if err != nil {
		return nil, mapAdminProductRepoErr(err)
	}
	return out, nil
}
