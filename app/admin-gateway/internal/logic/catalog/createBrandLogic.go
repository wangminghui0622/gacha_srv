// Code scaffolded by goctl. Safe to edit.

package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateBrandLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateBrandLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateBrandLogic {
	return &CreateBrandLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateBrandLogic) CreateBrand(req *catalogpb.CreateBrandRequest) (*catalogpb.Brand, error) {
	return l.svcCtx.CatalogRpc.CreateBrand(l.ctx, req)
}
