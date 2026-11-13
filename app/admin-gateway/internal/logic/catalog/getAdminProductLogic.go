package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAdminProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAdminProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAdminProductLogic {
	return &GetAdminProductLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetAdminProductLogic) GetAdminProduct(productID uint64) (*catalogpb.AdminProductDetail, error) {
	return l.svcCtx.CatalogRpc.GetAdminProduct(l.ctx, &catalogpb.GetAdminProductRequest{ProductId: productID})
}
