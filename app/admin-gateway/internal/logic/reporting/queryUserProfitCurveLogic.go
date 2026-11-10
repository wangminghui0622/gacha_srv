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

type QueryUserProfitCurveLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryUserProfitCurveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryUserProfitCurveLogic {
	return &QueryUserProfitCurveLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// QueryUserProfitCurve 用户盈亏曲线（reporting.metric:read）。
// 租户守卫照抄 server_v2 reporting_http.go：客户端传 tenantId 与会话不符 → 403
// TENANT_MISMATCH；否则绑定为会话租户。
func (l *QueryUserProfitCurveLogic) QueryUserProfitCurve(in *ops.QueryUserProfitCurveRequest) (*ops.QueryUserProfitCurveReply, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话主体")
	}
	if in.GetTenantId() != 0 && in.GetTenantId() != claims.TenantID {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTenantMismatch, "tenant_id 与会话不符")
	}
	in.TenantId = claims.TenantID
	return l.svcCtx.OpsRpc.QueryUserProfitCurve(l.ctx, in)
}
