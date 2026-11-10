// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package catalog

import (
	"context"

	catalogpb "gacha_srv/app/catalog/catalog"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveProductVariantRevisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// SaveProductVariantRevision 保存 SKU 变体修订（catalog.product:write，透传 catalog-rpc）。
func NewSaveProductVariantRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveProductVariantRevisionLogic {
	return &SaveProductVariantRevisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SaveProductVariantRevisionLogic) SaveProductVariantRevision(req *catalogpb.SaveProductVariantRevisionRequest) (*catalogpb.Product, error) {
	return l.svcCtx.CatalogRpc.SaveProductVariantRevision(l.ctx, req)
}
