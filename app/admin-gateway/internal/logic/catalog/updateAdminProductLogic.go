package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAdminProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAdminProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAdminProductLogic {
	return &UpdateAdminProductLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateAdminProductLogic) UpdateAdminProduct(req *catalogpb.UpdateAdminProductRequest) (*catalogpb.AdminProduct, error) {
	return l.svcCtx.CatalogRpc.UpdateAdminProduct(l.ctx, req)
}
