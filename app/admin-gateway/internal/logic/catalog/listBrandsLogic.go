// Code scaffolded by goctl. Safe to edit.

package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBrandsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListBrandsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBrandsLogic {
	return &ListBrandsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListBrandsLogic) ListBrands(req *catalogpb.ListBrandsRequest) (*catalogpb.ListBrandsReply, error) {
	return l.svcCtx.CatalogRpc.ListBrands(l.ctx, req)
}
