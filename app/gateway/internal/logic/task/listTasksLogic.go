package task

import (
	"context"

	"gacha_srv/app/task/task"
	taskClient "gacha_srv/app/task/taskClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTasksLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTasksLogic {
	return &ListTasksLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListTasks 任务列表（GET /tasks，公开；可选登录绑定）。匿名（无令牌）→ userId=0，
// task 侧按老项目语义返回 7003（userID==0 → ErrInvalidTask）。
func (l *ListTasksLogic) ListTasks(req *types.ListTasksQuery) (resp *types.ListTasksResp, err error) {
	var userId int64
	if uid, ok := middleware.UserIdFromCtx(l.ctx); ok {
		userId = uid
	}
	r, err := l.svcCtx.TaskRpc.ListTasks(l.ctx, &taskClient.ListTasksReq{
		UserId:    userId,
		Period:    task.TaskPeriod(req.Period),
		PageSize:  uint32(req.PageSize),
		PageToken: req.PageToken,
	})
	if err != nil {
		l.Errorf("ListTasks failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	return tasksToResp(r), nil
}
