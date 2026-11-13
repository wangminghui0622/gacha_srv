// Code scaffolded by goctl. Safe to edit.

package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateBrandLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateBrandLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateBrandLogic {
	return &UpdateBrandLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateBrandLogic) UpdateBrand(req *catalogpb.UpdateBrandRequest) (*catalogpb.Brand, error) {
	return l.svcCtx.CatalogRpc.UpdateBrand(l.ctx, req)
}
