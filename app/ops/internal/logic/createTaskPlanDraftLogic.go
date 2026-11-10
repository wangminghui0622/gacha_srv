package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
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

// CreateTaskPlanDraft 创建计划草稿（expected_latest_version CAS 守卫；
// 校验+哈希由 repo 内 TaskPlanPayload 完成，非法 → 7018）。
func (l *CreateTaskPlanDraftLogic) CreateTaskPlanDraft(in *ops.CreateTaskPlanDraftRequest) (*ops.TaskPlan, error) {
	plan := &ops.TaskPlan{
		Tasks: in.GetTasks(), Passes: in.GetPasses(), MemberTiers: in.GetMemberTiers(),
	}
	return l.svcCtx.PlanStore.CreateTaskPlanDraft(l.ctx, plan, in.GetExpectedLatestVersion())
}
