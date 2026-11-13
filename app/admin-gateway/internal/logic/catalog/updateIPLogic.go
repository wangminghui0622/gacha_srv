// Code scaffolded by goctl. Safe to edit.

package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateIPLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateIPLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateIPLogic {
	return &UpdateIPLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateIPLogic) UpdateIP(req *catalogpb.UpdateIPRequest) (*catalogpb.IP, error) {
	return l.svcCtx.CatalogRpc.UpdateIP(l.ctx, req)
}
