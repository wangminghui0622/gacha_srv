// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package task

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTaskPlanDraftLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTaskPlanDraftLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTaskPlanDraftLogic {
	return &CreateTaskPlanDraftLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateTaskPlanDraft 透传 ops-rpc（task.plan:write）。
func (l *CreateTaskPlanDraftLogic) CreateTaskPlanDraft(in *ops.CreateTaskPlanDraftRequest) (*ops.TaskPlan, error) {
	return l.svcCtx.OpsRpc.CreateTaskPlanDraft(l.ctx, in)
}
