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

// ApproveTaskPlanPublication 审批任务计划发布（task.plan:approve_publication；
// 审批人 = 会话主体 ActorID，不接受客户端传入——照抄 server_v2 task_http.go）。
func (l *ApproveTaskPlanPublicationLogic) ApproveTaskPlanPublication(in *ops.ApproveTaskPlanPublicationRequest) (*ops.TaskPlanPublicationApproval, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil || claims.ActorID == 0 {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话主体")
	}
	in.ApproverId = claims.ActorID
	return l.svcCtx.OpsRpc.ApproveTaskPlanPublication(l.ctx, in)
}
