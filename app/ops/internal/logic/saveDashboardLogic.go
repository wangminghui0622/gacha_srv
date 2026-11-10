package logic

import (
	"context"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/internal/svc"
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

// SaveDashboard 保存看板（server_v2 biz.SaveDashboard 同款：id=0 创建
// 强制 version=1&&DRAFT；否则按 expected_version CAS 更新）。
func (l *SaveDashboardLogic) SaveDashboard(in *ops.SaveDashboardRequest) (*ops.Dashboard, error) {
	d := &ops.Dashboard{
		DashboardId: in.GetDashboardId(), DashboardCode: in.GetDashboardCode(), Title: in.GetTitle(),
		TenantId: in.GetTenantId(), Widgets: in.GetWidgets(),
	}
	if err := core.ValidateDashboard(d); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, err.Error())
	}
	if d.GetDashboardId() == 0 {
		d.Version, d.Status = 1, ops.DashboardStatus_DASHBOARD_STATUS_DRAFT
	} else {
		d.Version = in.GetExpectedVersion()
		if d.GetVersion() == 0 {
			return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, "expected_version 非法")
		}
	}
	return l.svcCtx.ReportingStore.SaveDashboard(l.ctx, d)
}
