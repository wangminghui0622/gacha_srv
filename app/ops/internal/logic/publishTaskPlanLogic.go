package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishTaskPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishTaskPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishTaskPlanLogic {
	return &PublishTaskPlanLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PublishTaskPlan 发布计划（结构化奖励 → 7019；旧发布版退隐 2→3；
// head/版本/审批 CAS 全链路）。
func (l *PublishTaskPlanLogic) PublishTaskPlan(in *ops.PublishTaskPlanRequest) (*ops.TaskPlan, error) {
	return l.svcCtx.PlanStore.PublishTaskPlan(l.ctx, in.GetVersion(), in.GetExpectedConfigHash())
}
