// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package authorization

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PutPrincipalRolesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 配置主体角色（authorization.role:write，乐观锁）
func NewPutPrincipalRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutPrincipalRolesLogic {
	return &PutPrincipalRolesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PutPrincipalRolesLogic) PutPrincipalRoles(req *ops.PutPrincipalRolesRequest) (*ops.PrincipalRoles, error) {
	return l.svcCtx.OpsRpc.PutPrincipalRoles(l.ctx, req)
}
