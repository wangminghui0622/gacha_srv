// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package authorization

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PutRolePermissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 配置角色权限（authorization.role:write，整表替换）
func NewPutRolePermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutRolePermissionsLogic {
	return &PutRolePermissionsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PutRolePermissionsLogic) PutRolePermissions(req *ops.PutRolePermissionsRequest) (*ops.Role, error) {
	return l.svcCtx.OpsRpc.PutRolePermissions(l.ctx, req)
}
