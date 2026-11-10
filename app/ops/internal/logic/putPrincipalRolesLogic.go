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

type PutPrincipalRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPutPrincipalRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutPrincipalRolesLogic {
	return &PutPrincipalRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PutPrincipalRoles 全量替换账号角色（照抄 server_v2：expected_version 乐观锁，
// 版本存账号行 role_version，事务内删旧插新 + 版本 +1）。
func (l *PutPrincipalRolesLogic) PutPrincipalRoles(in *ops.PutPrincipalRolesRequest) (*ops.PrincipalRoles, error) {
	tenantID := defaultTenant(in.TenantId)
	if in.PrincipalId == 0 || in.ExpectedVersion == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "账号 ID 与期望版本必填")
	}
	account, err := l.svcCtx.AccountsModel.FindOne(l.ctx, in.PrincipalId)
	if repo.IsNotFound(err) || account.TenantId != tenantID {
		return nil, errorx.NewCodeError(errorx.CodeConsoleAccountNotFound, "账号不存在")
	}
	if err != nil {
		return nil, err
	}
	// 角色必须属于本租户
	if len(in.RoleIds) > 0 {
		var count int64
		if err = l.svcCtx.DB.WithContext(l.ctx).Model(&opsmodel.Roles{}).
			Where("`tenant_id` = ? AND `id` IN ?", tenantID, in.RoleIds).Count(&count).Error; err != nil {
			return nil, err
		}
		if count != int64(len(uniqueIDs(in.RoleIds))) {
			return nil, errorx.NewCodeError(errorx.CodeConsoleRoleNotFound, "角色不存在或不属于本租户")
		}
	}

	newVersion := in.ExpectedVersion + 1
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&opsmodel.ConsoleAccounts{}).
			Where("`id` = ? AND `tenant_id` = ? AND `role_version` = ?", in.PrincipalId, tenantID, in.ExpectedVersion).
			Update("role_version", newVersion)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errorx.NewCodeError(errorx.CodeConsoleRoleVersionConflict, "账号角色版本冲突")
		}
		if err := tx.Where("`account_id` = ?", in.PrincipalId).Delete(&opsmodel.PrincipalRoles{}).Error; err != nil {
			return err
		}
		for _, roleID := range in.RoleIds {
			if err := tx.Create(&opsmodel.PrincipalRoles{AccountId: in.PrincipalId, RoleId: roleID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &ops.PrincipalRoles{
		PrincipalId: in.PrincipalId, TenantId: tenantID,
		Version: newVersion, RoleIds: in.RoleIds,
	}, nil
}

func uniqueIDs(ids []uint64) []uint64 {
	seen := map[uint64]struct{}{}
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
