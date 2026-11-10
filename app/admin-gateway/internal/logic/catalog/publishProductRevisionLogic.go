// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package catalog

import (
	"context"

	catalogpb "gacha_srv/app/catalog/catalog"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishProductRevisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// PublishProductRevision 发布商品修订（catalog.product:publish，透传 catalog-rpc）。
func NewPublishProductRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishProductRevisionLogic {
	return &PublishProductRevisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PublishProductRevisionLogic) PublishProductRevision(req *catalogpb.PublishProductRevisionRequest) (*catalogpb.Product, error) {
	return l.svcCtx.CatalogRpc.PublishProductRevision(l.ctx, req)
}
