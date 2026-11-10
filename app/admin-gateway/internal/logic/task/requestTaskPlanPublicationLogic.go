// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package task

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

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

// RequestTaskPlanPublication 申请发布任务计划（task.plan:request_publication；
// 申请人 = 会话主体 ActorID，不接受客户端传入——照抄 server_v2 task_http.go）。
func (l *RequestTaskPlanPublicationLogic) RequestTaskPlanPublication(in *ops.RequestTaskPlanPublicationRequest) (*ops.TaskPlanPublicationApproval, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil || claims.ActorID == 0 {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话主体")
	}
	in.RequesterId = claims.ActorID
	return l.svcCtx.OpsRpc.RequestTaskPlanPublication(l.ctx, in)
}
