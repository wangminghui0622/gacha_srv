package logic

import (
	"context"

	"gacha_srv/app/task/internal/core/task"
	"gacha_srv/app/task/internal/repo"
	"gacha_srv/app/task/internal/svc"
	taskpb "gacha_srv/app/task/task"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ApplyTaskProgressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplyTaskProgressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyTaskProgressLogic {
	return &ApplyTaskProgressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ApplyTaskProgress 进度事件应用（内部 RPC，D4：网关购买挂钩调用，无 HTTP 路由）。
// 老项目 ApplyContext：task_event 幂等去重 → 匹配 metric 任务 amount 累加 +
// DRAW_SPEND_MINOR 时通行证 XP（封顶）与会员 spend。SIGN_IN/非法 metric/amount≤0 → 3054。
func (l *ApplyTaskProgressLogic) ApplyTaskProgress(in *taskpb.ApplyTaskProgressReq) (*taskpb.ApplyTaskProgressResp, error) {
	if in.UserId <= 0 || in.EventId == "" || len(in.EventId) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeTaskInvalid, "任务参数无效")
	}
	if in.Amount <= 0 || in.Metric == taskpb.TaskMetric_TASK_METRIC_UNSPECIFIED || in.Metric == taskpb.TaskMetric_TASK_METRIC_SIGN_IN {
		return nil, errorx.NewCodeError(errorx.CodeApplyProgressInvalid, "进度事件非法")
	}
	snapshot, err := loadTaskSnapshot(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	delta, err := task.ApplyDeltaFor(snapshot, uint64(in.UserId), in.EventId, in.Metric, in.Amount, taskNow())
	if err != nil {
		return nil, mapApplyErr(err)
	}
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		_, err := repo.SaveApply(l.ctx, tx, delta)
		return err
	})
	if err != nil {
		return nil, mapApplyErr(err)
	}
	return &taskpb.ApplyTaskProgressResp{}, nil
}
