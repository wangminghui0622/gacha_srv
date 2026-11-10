// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package authorization

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 修改角色（authorization.role:write，乐观锁）
func NewUpdateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRoleLogic {
	return &UpdateRoleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateRoleLogic) UpdateRole(req *ops.UpdateRoleRequest) (*ops.Role, error) {
	return l.svcCtx.OpsRpc.UpdateRole(l.ctx, req)
}
