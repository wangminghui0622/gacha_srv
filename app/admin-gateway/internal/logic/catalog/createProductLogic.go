// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package catalog

import (
	"context"

	catalogpb "gacha_srv/app/catalog/catalog"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// CreateProduct 创建商品（catalog.product:create，透传 catalog-rpc）。
func NewCreateProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProductLogic {
	return &CreateProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateProductLogic) CreateProduct(req *catalogpb.CreateProductRequest) (*catalogpb.Product, error) {
	return l.svcCtx.CatalogRpc.CreateProduct(l.ctx, req)
}
