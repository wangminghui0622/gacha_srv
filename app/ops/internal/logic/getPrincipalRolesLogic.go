package logic

import (
	"context"

	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPrincipalRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPrincipalRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPrincipalRolesLogic {
	return &GetPrincipalRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetPrincipalRoles 账号的角色绑定（version = 账号行 role_version，乐观锁用）。
func (l *GetPrincipalRolesLogic) GetPrincipalRoles(in *ops.GetPrincipalRolesRequest) (*ops.PrincipalRoles, error) {
	tenantID := defaultTenant(in.TenantId)
	if in.PrincipalId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "账号 ID 必填")
	}
	account, err := l.svcCtx.AccountsModel.FindOne(l.ctx, in.PrincipalId)
	if repo.IsNotFound(err) || account.TenantId != tenantID {
		return nil, errorx.NewCodeError(errorx.CodeConsoleAccountNotFound, "账号不存在")
	}
	if err != nil {
		return nil, err
	}
	var roleIDs []uint64
	err = l.svcCtx.DB.WithContext(l.ctx).Table("`principal_roles`").
		Where("`account_id` = ?", in.PrincipalId).
		Order("`role_id`").
		Pluck("role_id", &roleIDs).Error
	if err != nil {
		return nil, err
	}
	return &ops.PrincipalRoles{
		PrincipalId: in.PrincipalId, TenantId: tenantID,
		Version: account.RoleVersion, RoleIds: roleIDs,
	}, nil
}
