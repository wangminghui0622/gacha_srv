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

type SaveDashboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveDashboardLogic {
	return &SaveDashboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SaveDashboard 保存看板（reporting.dashboard:write）；
// tenant 绑定为会话租户（照抄 server_v2 reporting_http.go，不接受客户端传入）。
func (l *SaveDashboardLogic) SaveDashboard(in *ops.SaveDashboardRequest) (*ops.Dashboard, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话主体")
	}
	in.TenantId = claims.TenantID
	return l.svcCtx.OpsRpc.SaveDashboard(l.ctx, in)
}
