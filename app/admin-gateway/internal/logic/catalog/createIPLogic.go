// Code scaffolded by goctl. Safe to edit.

package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateIPLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateIPLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateIPLogic {
	return &CreateIPLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateIPLogic) CreateIP(req *catalogpb.CreateIPRequest) (*catalogpb.IP, error) {
	return l.svcCtx.CatalogRpc.CreateIP(l.ctx, req)
}
