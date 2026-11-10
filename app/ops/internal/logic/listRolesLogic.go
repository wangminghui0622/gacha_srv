package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRolesLogic {
	return &ListRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListRoles 角色列表（含各角色权限行）。
func (l *ListRolesLogic) ListRoles(in *ops.ListRolesRequest) (*ops.ListRolesReply, error) {
	tenantID := defaultTenant(in.TenantId)
	var roles []*opsmodel.Roles
	err := l.svcCtx.DB.WithContext(l.ctx).Model(&opsmodel.Roles{}).
		Where("`tenant_id` = ?", tenantID).Order("`id`").Find(&roles).Error
	if err != nil {
		return nil, err
	}
	var perms []*opsmodel.RolePermissions
	err = l.svcCtx.DB.WithContext(l.ctx).Model(&opsmodel.RolePermissions{}).
		Joins("JOIN `roles` AS r ON r.id = role_permissions.role_id AND r.tenant_id = ?", tenantID).
		Order("role_permissions.`resource`, role_permissions.`action`").
		Find(&perms).Error
	if err != nil {
		return nil, err
	}
	byRole := map[uint64][]*opsmodel.RolePermissions{}
	for _, p := range perms {
		byRole[p.RoleId] = append(byRole[p.RoleId], p)
	}
	reply := &ops.ListRolesReply{}
	for _, role := range roles {
		reply.Roles = append(reply.Roles, roleToProto(role, byRole[role.Id]))
	}
	return reply, nil
}
