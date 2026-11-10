// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package catalog

import (
	"context"

	catalogpb "gacha_srv/app/catalog/catalog"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProductRevisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// GetProductRevision 查看商品修订（catalog.product:read，透传 catalog-rpc）。
func NewGetProductRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProductRevisionLogic {
	return &GetProductRevisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProductRevisionLogic) GetProductRevision(req *catalogpb.GetProductRevisionRequest) (*catalogpb.Product, error) {
	return l.svcCtx.CatalogRpc.GetProductRevision(l.ctx, req)
}
