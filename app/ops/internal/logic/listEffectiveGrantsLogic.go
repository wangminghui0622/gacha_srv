package logic

import (
	"context"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListEffectiveGrantsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListEffectiveGrantsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListEffectiveGrantsLogic {
	return &ListEffectiveGrantsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListEffectiveGrants 有效权限（前端动态路由用）：allow 规则按角色过滤，
// 条件/限定操作的规则跳过，通配 DENY 摘除，按资源分组。
func (l *ListEffectiveGrantsLogic) ListEffectiveGrants(in *ops.ListEffectiveGrantsRequest) (*ops.ListEffectiveGrantsReply, error) {
	if in.ActorId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "actor 必填")
	}
	tenantID := defaultTenant(in.TenantId)
	roleCodes := accountRoleCodes(l.ctx, l.svcCtx, tenantID, in.ActorId)
	rules, err := l.svcCtx.AuthzStore.LoadRulesForRoles(l.ctx, tenantID, roleCodes)
	if err != nil {
		return nil, err
	}
	grants, nodes := core.EffectiveGrants(rules, roleCodes)
	return &ops.ListEffectiveGrantsReply{Grants: grants, Nodes: nodes}, nil
}
