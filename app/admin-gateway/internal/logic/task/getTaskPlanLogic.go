// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package task

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTaskPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTaskPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTaskPlanLogic {
	return &GetTaskPlanLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetTaskPlan 透传 ops-rpc（task.plan:read；version=0 → 最新）。
func (l *GetTaskPlanLogic) GetTaskPlan(in *ops.GetTaskPlanRequest) (*ops.TaskPlan, error) {
	return l.svcCtx.OpsRpc.GetTaskPlan(l.ctx, in)
}
