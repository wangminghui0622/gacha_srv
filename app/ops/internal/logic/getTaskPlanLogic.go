package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
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

// GetTaskPlan 读取计划（version=0 → 最新版本）。
func (l *GetTaskPlanLogic) GetTaskPlan(in *ops.GetTaskPlanRequest) (*ops.TaskPlan, error) {
	return l.svcCtx.PlanStore.GetTaskPlan(l.ctx, in.GetVersion())
}
