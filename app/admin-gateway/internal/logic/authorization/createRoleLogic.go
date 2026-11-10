// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package authorization

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建角色（authorization.role:write）
func NewCreateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRoleLogic {
	return &CreateRoleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateRoleLogic) CreateRole(req *ops.CreateRoleRequest) (*ops.Role, error) {
	return l.svcCtx.OpsRpc.CreateRole(l.ctx, req)
}
