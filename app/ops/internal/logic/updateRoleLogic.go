package logic

import (
	"context"

	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRoleLogic {
	return &UpdateRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateRole 修改角色（名称/启停；expected_version 乐观锁，版本冲突 7011）。
func (l *UpdateRoleLogic) UpdateRole(in *ops.UpdateRoleRequest) (*ops.Role, error) {
	tenantID := defaultTenant(in.TenantId)
	if in.RoleId == 0 || in.ExpectedVersion == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "角色 ID 与期望版本必填")
	}
	name := trimNonEmpty(in.Name)
	if name == "" || len(name) > 128 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "名称(1-128)必填")
	}
	if _, err := l.svcCtx.RolesModel.FindOne(l.ctx, in.RoleId); repo.IsNotFound(err) {
		return nil, errorx.NewCodeError(errorx.CodeConsoleRoleNotFound, "角色不存在")
	} else if err != nil {
		return nil, err
	}
	enabled := int64(0)
	if in.Enabled {
		enabled = 1
	}
	res := l.svcCtx.DB.WithContext(l.ctx).Model(&opsmodel.Roles{}).
		Where("`id` = ? AND `tenant_id` = ? AND `version` = ?", in.RoleId, tenantID, in.ExpectedVersion).
		Updates(map[string]any{"name": name, "enabled": enabled, "version": in.ExpectedVersion + 1})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, errorx.NewCodeError(errorx.CodeConsoleRoleVersionConflict, "角色版本冲突")
	}
	role, perms, _, err := loadRoleWithPerms(l.ctx, l.svcCtx, tenantID, in.RoleId)
	if err != nil {
		return nil, err
	}
	return roleToProto(role, perms), nil
}
