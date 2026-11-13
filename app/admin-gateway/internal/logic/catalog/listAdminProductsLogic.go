package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAdminProductsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAdminProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAdminProductsLogic {
	return &ListAdminProductsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListAdminProductsLogic) ListAdminProducts(req *catalogpb.ListAdminProductsRequest) (*catalogpb.ListAdminProductsReply, error) {
	return l.svcCtx.CatalogRpc.ListAdminProducts(l.ctx, req)
}
