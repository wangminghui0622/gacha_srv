// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package authorization

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRoleMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 角色成员（authorization.role:read）
func NewListRoleMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRoleMembersLogic {
	return &ListRoleMembersLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListRoleMembersLogic) ListRoleMembers(req *ops.ListRoleMembersRequest) (*ops.ListRoleMembersReply, error) {
	return l.svcCtx.OpsRpc.ListRoleMembers(l.ctx, req)
}
