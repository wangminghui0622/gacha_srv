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

type PublishDashboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishDashboardLogic {
	return &PublishDashboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PublishDashboard 发布看板（reporting.dashboard:publish）；
// tenant 绑定为会话租户（照抄 server_v2 reporting_http.go）。
func (l *PublishDashboardLogic) PublishDashboard(in *ops.PublishDashboardRequest) (*ops.Dashboard, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话主体")
	}
	in.TenantId = claims.TenantID
	return l.svcCtx.OpsRpc.PublishDashboard(l.ctx, in)
}
