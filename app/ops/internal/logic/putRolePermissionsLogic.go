package logic

import (
	"context"

	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type PutRolePermissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPutRolePermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutRolePermissionsLogic {
	return &PutRolePermissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PutRolePermissions 全量替换角色权限（照抄 server_v2 语义：
// expected_version 乐观锁 + 权限目录校验 + 事务内删旧插新 + 版本 +1）。
func (l *PutRolePermissionsLogic) PutRolePermissions(in *ops.PutRolePermissionsRequest) (*ops.Role, error) {
	tenantID := defaultTenant(in.TenantId)
	if in.RoleId == 0 || in.ExpectedVersion == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "角色 ID 与期望版本必填")
	}
	// 权限目录校验（未知资源/动作 → 7012）
	for _, p := range in.Permissions {
		if err := repo.EnsureConsolePermission(p.Resource, p.Action); err != nil {
			return nil, errorx.NewCodeError(errorx.CodeConsoleUnknownPermission, err.Error())
		}
		if p.DataScope < ops.DataScope_DATA_SCOPE_SELF || p.DataScope > ops.DataScope_DATA_SCOPE_ALL {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "数据范围取值 1-6")
		}
		if p.ApprovalMode < ops.ApprovalMode_APPROVAL_MODE_NONE || p.ApprovalMode > ops.ApprovalMode_APPROVAL_MODE_DUAL_CONTROL {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "审批模式取值 1-3")
		}
	}
	if _, err := l.svcCtx.RolesModel.FindOne(l.ctx, in.RoleId); repo.IsNotFound(err) {
		return nil, errorx.NewCodeError(errorx.CodeConsoleRoleNotFound, "角色不存在")
	} else if err != nil {
		return nil, err
	}

	err := l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		// 乐观锁 + 版本 +1（与更新同事务原子完成）
		res := tx.Model(&opsmodel.Roles{}).
			Where("`id` = ? AND `tenant_id` = ? AND `version` = ?", in.RoleId, tenantID, in.ExpectedVersion).
			Update("version", in.ExpectedVersion+1)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errorx.NewCodeError(errorx.CodeConsoleRoleVersionConflict, "角色版本冲突")
		}
		// 删旧插新
		if err := tx.Where("`role_id` = ?", in.RoleId).Delete(&opsmodel.RolePermissions{}).Error; err != nil {
			return err
		}
		for _, p := range in.Permissions {
			row := &opsmodel.RolePermissions{
				RoleId: in.RoleId, Resource: p.Resource, Action: p.Action,
				DataScope: int64(p.DataScope), ApprovalMode: int64(p.ApprovalMode),
				ConditionExpr: p.Condition,
				AllowedFields: stringsToJSON(p.AllowedFields), DeniedFields: stringsToJSON(p.DeniedFields),
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	role, perms, _, err := loadRoleWithPerms(l.ctx, l.svcCtx, tenantID, in.RoleId)
	if err != nil {
		return nil, err
	}
	return roleToProto(role, perms), nil
}
