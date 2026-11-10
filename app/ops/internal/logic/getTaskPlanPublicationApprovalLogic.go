package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
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

// GetTaskPlanPublicationApproval 读取发布审批行。
func (l *GetTaskPlanPublicationApprovalLogic) GetTaskPlanPublicationApproval(in *ops.GetTaskPlanPublicationApprovalRequest) (*ops.TaskPlanPublicationApproval, error) {
	return l.svcCtx.PlanStore.GetTaskPlanPublicationApproval(l.ctx, in.GetVersion())
}
