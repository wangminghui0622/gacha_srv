// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package task

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTaskPlanPublicationApprovalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTaskPlanPublicationApprovalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTaskPlanPublicationApprovalLogic {
	return &GetTaskPlanPublicationApprovalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetTaskPlanPublicationApproval 透传 ops-rpc（task.plan:read；
// 无审批行 → 404，是正常空态）。
func (l *GetTaskPlanPublicationApprovalLogic) GetTaskPlanPublicationApproval(in *ops.GetTaskPlanPublicationApprovalRequest) (*ops.TaskPlanPublicationApproval, error) {
	return l.svcCtx.OpsRpc.GetTaskPlanPublicationApproval(l.ctx, in)
}
