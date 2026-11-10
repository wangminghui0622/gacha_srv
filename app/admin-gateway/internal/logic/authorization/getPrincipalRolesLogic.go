// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package authorization

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPrincipalRolesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查看主体角色（authorization.role:read）
func NewGetPrincipalRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPrincipalRolesLogic {
	return &GetPrincipalRolesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetPrincipalRolesLogic) GetPrincipalRoles(req *ops.GetPrincipalRolesRequest) (*ops.PrincipalRoles, error) {
	return l.svcCtx.OpsRpc.GetPrincipalRoles(l.ctx, req)
}
