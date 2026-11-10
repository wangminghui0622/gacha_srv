package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDashboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDashboardLogic {
	return &GetDashboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetDashboard 读取看板（含 widgets）。
func (l *GetDashboardLogic) GetDashboard(in *ops.GetDashboardRequest) (*ops.Dashboard, error) {
	if in.GetTenantId() == 0 || in.GetDashboardId() == 0 {
		return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, "参数非法（tenant/dashboard）")
	}
	return l.svcCtx.ReportingStore.GetDashboard(l.ctx, in.GetTenantId(), in.GetDashboardId())
}
