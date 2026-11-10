package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"

	"github.com/zeromicro/go-zero/core/logx"
)

type RequestTaskPlanPublicationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRequestTaskPlanPublicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestTaskPlanPublicationLogic {
	return &RequestTaskPlanPublicationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RequestTaskPlanPublication 提请发布（requester_id 由管理网关以会话主体覆写；
// 同申请人+同哈希重放先前审批）。
func (l *RequestTaskPlanPublicationLogic) RequestTaskPlanPublication(in *ops.RequestTaskPlanPublicationRequest) (*ops.TaskPlanPublicationApproval, error) {
	return l.svcCtx.PlanStore.RequestTaskPlanPublication(l.ctx, in.GetVersion(), in.GetExpectedConfigHash(), in.GetRequesterId())
}
