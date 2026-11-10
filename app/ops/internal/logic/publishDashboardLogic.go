package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
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

// PublishDashboard 发布看板（tenant/id/version 校验 + CAS）。
func (l *PublishDashboardLogic) PublishDashboard(in *ops.PublishDashboardRequest) (*ops.Dashboard, error) {
	if in.GetTenantId() == 0 || in.GetDashboardId() == 0 || in.GetExpectedVersion() == 0 {
		return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, "发布参数非法（tenant/dashboard/version）")
	}
	return l.svcCtx.ReportingStore.PublishDashboard(l.ctx, in.GetTenantId(), in.GetDashboardId(), in.GetExpectedVersion())
}
