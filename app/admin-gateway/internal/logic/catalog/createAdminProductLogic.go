package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAdminProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateAdminProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAdminProductLogic {
	return &CreateAdminProductLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateAdminProductLogic) CreateAdminProduct(req *catalogpb.CreateAdminProductRequest) (*catalogpb.AdminProduct, error) {
	return l.svcCtx.CatalogRpc.CreateAdminProduct(l.ctx, req)
}
