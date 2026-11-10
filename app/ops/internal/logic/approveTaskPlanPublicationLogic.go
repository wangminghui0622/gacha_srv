package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApproveTaskPlanPublicationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApproveTaskPlanPublicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveTaskPlanPublicationLogic {
	return &ApproveTaskPlanPublicationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ApproveTaskPlanPublication 批准发布（approver_id 由管理网关以会话主体覆写；
// 自批/已由他人批准 → 冲突；APPROVED 同审批人重放）。
func (l *ApproveTaskPlanPublicationLogic) ApproveTaskPlanPublication(in *ops.ApproveTaskPlanPublicationRequest) (*ops.TaskPlanPublicationApproval, error) {
	return l.svcCtx.PlanStore.ApproveTaskPlanPublication(l.ctx, in.GetVersion(), in.GetExpectedConfigHash(), in.GetApproverId())
}
