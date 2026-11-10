// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package task

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
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

// PublishTaskPlan 透传 ops-rpc（task.plan:publish）。
func (l *PublishTaskPlanLogic) PublishTaskPlan(in *ops.PublishTaskPlanRequest) (*ops.TaskPlan, error) {
	return l.svcCtx.OpsRpc.PublishTaskPlan(l.ctx, in)
}
