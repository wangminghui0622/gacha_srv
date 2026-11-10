package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoleLogic {
	return &GetRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetRole 角色详情（含权限行）。
func (l *GetRoleLogic) GetRole(in *ops.GetRoleRequest) (*ops.Role, error) {
	tenantID := defaultTenant(in.TenantId)
	if in.RoleId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "角色 ID 必填")
	}
	role, perms, ok, err := loadRoleWithPerms(l.ctx, l.svcCtx, tenantID, in.RoleId)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errorx.NewCodeError(errorx.CodeConsoleRoleNotFound, "角色不存在")
	}
	return roleToProto(role, perms), nil
}
