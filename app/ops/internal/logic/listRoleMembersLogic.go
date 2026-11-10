package logic

import (
	"context"

	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRoleMembersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRoleMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRoleMembersLogic {
	return &ListRoleMembersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListRoleMembers 角色成员分页（account_id 游标，next_principal_id 语义照抄 server_v2）。
func (l *ListRoleMembersLogic) ListRoleMembers(in *ops.ListRoleMembersRequest) (*ops.ListRoleMembersReply, error) {
	tenantID := defaultTenant(in.TenantId)
	if in.RoleId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "角色 ID 必填")
	}
	if _, err := l.svcCtx.RolesModel.FindOne(l.ctx, in.RoleId); repo.IsNotFound(err) {
		return nil, errorx.NewCodeError(errorx.CodeConsoleRoleNotFound, "角色不存在")
	} else if err != nil {
		return nil, err
	}
	pageSize := in.PageSize
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var rows []struct {
		AccountId   uint64
		DisplayName string
	}
	err := l.svcCtx.DB.WithContext(l.ctx).
		Table("`principal_roles` AS pr").
		Select("pr.account_id, COALESCE(a.display_name, '') AS display_name").
		Joins("JOIN `console_accounts` AS a ON a.id = pr.account_id AND a.tenant_id = ?", tenantID).
		Where("pr.role_id = ? AND pr.account_id > ?", in.RoleId, in.AfterPrincipalId).
		Order("pr.account_id").
		Limit(int(pageSize) + 1).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	reply := &ops.ListRoleMembersReply{}
	for i, row := range rows {
		if i == int(pageSize) {
			reply.NextPrincipalId = rows[i-1].AccountId
			break
		}
		reply.Members = append(reply.Members, &ops.RoleMember{
			PrincipalId: row.AccountId, DisplayName: row.DisplayName,
		})
	}
	return reply, nil
}
