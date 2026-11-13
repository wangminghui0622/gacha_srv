// Code scaffolded by goctl. Safe to edit.

package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListIPsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListIPsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListIPsLogic {
	return &ListIPsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListIPsLogic) ListIPs(req *catalogpb.ListIPsRequest) (*catalogpb.ListIPsReply, error) {
	return l.svcCtx.CatalogRpc.ListIPs(l.ctx, req)
}
