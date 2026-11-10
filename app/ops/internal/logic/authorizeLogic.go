package logic

import (
	"context"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthorizeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthorizeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthorizeLogic {
	return &AuthorizeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Authorize 授权决策（照抄 server_v2 authorization Authorize 语义）：
// 角色编码服务端权威解析（principal_roles 联查，JWT 快照仅供参考）；
// 规则 = 启用角色权限行；匹配 operation_id/resource/action/角色 + CEL 条件；
// DENY 优先；多规则 data_scope 取最严、allowed_fields 取交、denied_fields 取并、approval_mode 取最大。
func (l *AuthorizeLogic) Authorize(in *ops.AuthorizeRequest) (*ops.AuthorizationDecision, error) {
	if in.RequestId == "" {
		return nil, errorx.NewCodeError(errorx.CodeConsoleRequestIdMissing, "缺少请求 ID")
	}
	if in.ActorId == 0 || in.Resource == "" || in.Action == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "actor/resource/action 必填")
	}
	tenantID := defaultTenant(in.TenantId)

	// 服务端权威角色解析（账号停用/不存在 → 无角色 → 默认拒绝）
	roleCodes := accountRoleCodes(l.ctx, l.svcCtx, tenantID, in.ActorId)
	rules, err := l.svcCtx.AuthzStore.LoadRulesForRoles(l.ctx, tenantID, roleCodes)
	if err != nil {
		return nil, err
	}
	decision, err := l.svcCtx.Authz.Authorize(&core.AuthzRequest{
		RequestID: in.RequestId, ActorID: in.ActorId, TenantID: tenantID, RoleCodes: roleCodes,
		Resource: in.Resource, Action: in.Action, OperationID: in.OperationId,
		Subject: attrsToMap(in.Subject), ResourceAttributes: attrsToMap(in.ResourceAttributes),
		Context: attrsToMap(in.Context),
	}, rules)
	if err != nil {
		return nil, err
	}
	return &ops.AuthorizationDecision{
		DecisionId: decision.DecisionID, Allowed: decision.Allowed, ReasonCode: decision.ReasonCode,
		MatchedRuleIds: decision.MatchedRuleIDs, DataScope: decision.DataScope,
		AllowedFields: decision.AllowedFields, DeniedFields: decision.DeniedFields,
		ApprovalMode: decision.ApprovalMode,
	}, nil
}
