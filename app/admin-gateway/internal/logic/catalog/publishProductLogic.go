// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package catalog

import (
	"context"

	catalogpb "gacha_srv/app/catalog/catalog"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// PublishProduct 首次发布商品（catalog.product:publish，透传 catalog-rpc）。
func NewPublishProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishProductLogic {
	return &PublishProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PublishProductLogic) PublishProduct(req *catalogpb.PublishProductRequest) (*catalogpb.Product, error) {
	return l.svcCtx.CatalogRpc.PublishProduct(l.ctx, req)
}
