// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package authorization

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPermissionCatalogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查看权限目录（authorization.role:read）
func NewListPermissionCatalogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPermissionCatalogLogic {
	return &ListPermissionCatalogLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListPermissionCatalogLogic) ListPermissionCatalog() (*ops.ListPermissionCatalogReply, error) {
	return l.svcCtx.OpsRpc.ListPermissionCatalog(l.ctx, &ops.ListPermissionCatalogRequest{})
}
