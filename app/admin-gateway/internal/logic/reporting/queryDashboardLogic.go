// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package reporting

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryDashboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryDashboardLogic {
	return &QueryDashboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// QueryDashboard 查询看板（reporting.dashboard:query）。
// 守卫照抄 server_v2 reporting_http.go：tenant 绑定为会话租户；
// 授权决策数据范围须 ORGANIZATION/ALL，否则 403 DATA_SCOPE_DENIED。
func (l *QueryDashboardLogic) QueryDashboard(in *ops.QueryDashboardRequest) (*ops.QueryDashboardReply, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话主体")
	}
	decision := restx.DecisionFrom(l.ctx)
	if decision == nil || (decision.DataScope != ops.DataScope_DATA_SCOPE_ORGANIZATION &&
		decision.DataScope != ops.DataScope_DATA_SCOPE_ALL) {
		return nil, errorx.NewCodeError(errorx.CodeConsoleDataScopeDenied, "报表查询需要组织或全局数据范围")
	}
	in.TenantId = claims.TenantID
	return l.svcCtx.OpsRpc.QueryDashboard(l.ctx, in)
}
