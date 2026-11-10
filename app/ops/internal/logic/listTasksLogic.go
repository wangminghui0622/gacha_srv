package logic

import (
	"context"
	"fmt"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTasksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTasksLogic {
	return &ListTasksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListTasks 玩家任务视图（进度管线不在本仓：progress 恒 0/claimed 0/IN_PROGRESS）。
// 游标语义照抄 server_v2 listFromState：token 为上一个 task_id，取 id>token 起；
// 取满 pageSize 且还有匹配任务时返回 next_page_token。
func (l *ListTasksLogic) ListTasks(in *ops.ListTasksRequest) (*ops.ListTasksReply, error) {
	if in.GetUserId() == 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskPlanInvalid, "user_id 非法")
	}
	pageSize := int(in.GetPageSize())
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	tasks, err := l.svcCtx.PlanStore.ListPublishedTasks(l.ctx)
	if err != nil {
		return nil, err
	}
	start := 0
	if token := in.GetPageToken(); token != "" {
		var tokenID uint64
		if _, err := fmt.Sscanf(token, "%d", &tokenID); err != nil {
			return nil, errorx.NewCodeError(errorx.CodeTaskPlanInvalid, "page_token 非法")
		}
		start = len(tasks)
		for i, task := range tasks {
			if task.GetTaskId() > tokenID {
				start = i
				break
			}
		}
	}
	views := make([]*ops.TaskView, 0, pageSize)
	var last uint64
	for _, task := range tasks[start:] {
		if period := in.GetPeriod(); period != ops.TaskPeriod_TASK_PERIOD_UNSPECIFIED && task.GetPeriod() != period {
			continue
		}
		views = append(views, &ops.TaskView{
			Definition: task, Progress: 0, Claimed: 0,
			State: ops.TaskProgressState_TASK_PROGRESS_STATE_IN_PROGRESS,
		})
		last = task.GetTaskId()
		if len(views) == pageSize {
			break
		}
	}
	next := ""
	if len(views) == pageSize {
		for _, task := range tasks {
			if task.GetTaskId() > last && (in.GetPeriod() == ops.TaskPeriod_TASK_PERIOD_UNSPECIFIED || task.GetPeriod() == in.GetPeriod()) {
				next = fmt.Sprint(last)
				break
			}
		}
	}
	return &ops.ListTasksReply{Tasks: views, NextPageToken: next}, nil
}
