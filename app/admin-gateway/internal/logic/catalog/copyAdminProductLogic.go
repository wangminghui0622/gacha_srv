package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type CopyAdminProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCopyAdminProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CopyAdminProductLogic {
	return &CopyAdminProductLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CopyAdminProductLogic) CopyAdminProduct(req *catalogpb.CopyAdminProductRequest) (*catalogpb.AdminProduct, error) {
	return l.svcCtx.CatalogRpc.CopyAdminProduct(l.ctx, req)
}
