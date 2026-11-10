// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package catalog

import (
	"context"

	catalogpb "gacha_srv/app/catalog/catalog"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveProductRevisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// SaveProductRevision 保存商品修订（catalog.product:write；同内容幂等，透传 catalog-rpc）。
func NewSaveProductRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveProductRevisionLogic {
	return &SaveProductRevisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SaveProductRevisionLogic) SaveProductRevision(req *catalogpb.SaveProductRevisionRequest) (*catalogpb.Product, error) {
	return l.svcCtx.CatalogRpc.SaveProductRevision(l.ctx, req)
}
