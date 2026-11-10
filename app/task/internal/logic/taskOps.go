package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/task/internal/core/task"
	"gacha_srv/app/task/internal/repo"
	"gacha_srv/app/task/internal/svc"
	taskpb "gacha_srv/app/task/task"
	"gacha_srv/pkg/utils/errorx"
)

// M15c task 逻辑层共享助手：计划快照加载（D2 跨库直读）与任务域错误映射（D6）。
// 老项目哨兵 → errorx 映射：ErrTaskNotFound→404；ErrTaskNotReady/Finished 按端点
// 上下文归入 3046-3052；乐观锁冲突 → 3053；进度事件非法 → 3054。

var taskLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}()

// taskNow 上海时区当前时刻（周期键/日历日判定基准）
func taskNow() time.Time { return time.Now().In(taskLocation) }

// loadTaskSnapshot 跨库只读已发布计划并重建快照（gacha_ops.task_plan_head.current_version
// → task_plan_version status=2 → decode 校验 config_hash）
func loadTaskSnapshot(ctx context.Context, svcCtx *svc.ServiceContext) (task.Snapshot, error) {
	plan, err := repo.LoadPublishedPlan(ctx, svcCtx.OpsDB)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			return task.Snapshot{}, errorx.NewCodeError(errorx.CodeTaskPlanNotFound, "任务计划不存在")
		}
		return task.Snapshot{}, err
	}
	snapshot, err := task.SnapshotFromOpsPlan(plan)
	if err != nil {
		return task.Snapshot{}, errorx.NewCodeError(errorx.CodeTaskPlanConflict, "任务计划校验失败")
	}
	return snapshot, nil
}

// findTaskDef 按 ID+Version 匹配任务定义（老项目 ClaimContext/SignInContext 同款线性查找）
func findTaskDef(snapshot task.Snapshot, taskID, version uint64) (task.Definition, bool) {
	for _, def := range snapshot.Tasks {
		if def.ID == taskID && def.Version == version {
			return def, true
		}
	}
	return task.Definition{}, false
}

// findSignInDef 按 ID+Version 匹配 SIGN_IN 任务（老项目 SignInContext 同款）
func findSignInDef(snapshot task.Snapshot, taskID, version uint64) (task.Definition, bool) {
	def, ok := findTaskDef(snapshot, taskID, version)
	return def, ok && def.Metric == taskpb.TaskMetric_TASK_METRIC_SIGN_IN
}

// mapClaimTaskErr 任务领取错误映射：NotReady→3046、Finished→3047、Invalid→3048、冲突→3053
func mapClaimTaskErr(err error) error {
	switch {
	case errors.Is(err, task.ErrTaskNotReady):
		return errorx.NewCodeError(errorx.CodeTaskNotReady, "任务未达标")
	case errors.Is(err, task.ErrTaskFinished):
		return errorx.NewCodeError(errorx.CodeTaskFinished, "任务已领完")
	case errors.Is(err, task.ErrInvalidTask):
		return errorx.NewCodeError(errorx.CodeTaskInvalid, "任务领取重放冲突")
	default:
		return mapTaskStateErr(err)
	}
}

// mapPassErr 通行证错误映射：NotReady→3049、Finished(槽位)→3050、Invalid→3048、冲突→3053
func mapPassErr(err error) error {
	switch {
	case errors.Is(err, task.ErrTaskNotReady):
		return errorx.NewCodeError(errorx.CodePassNotReady, "通行证领取条件未满足")
	case errors.Is(err, task.ErrTaskFinished):
		return errorx.NewCodeError(errorx.CodePassSlotClaimed, "通行证槽位已领")
	case errors.Is(err, task.ErrInvalidTask):
		return errorx.NewCodeError(errorx.CodeTaskInvalid, "通行证领取重放冲突")
	default:
		return mapTaskStateErr(err)
	}
}

// mapMemberErr 会员错误映射：NotReady→3051、Finished(档位)→3052、Invalid→3048、冲突→3053
func mapMemberErr(err error) error {
	switch {
	case errors.Is(err, task.ErrTaskNotReady):
		return errorx.NewCodeError(errorx.CodeMemberNotReady, "会员消费未达档位")
	case errors.Is(err, task.ErrTaskFinished):
		return errorx.NewCodeError(errorx.CodeMemberClaimed, "会员档已领")
	case errors.Is(err, task.ErrInvalidTask):
		return errorx.NewCodeError(errorx.CodeTaskInvalid, "会员领取重放冲突")
	default:
		return mapTaskStateErr(err)
	}
}

// mapSignInErr 签到错误映射：Finished(连续天数溢出)→3047、冲突→3053
func mapSignInErr(err error) error {
	if errors.Is(err, task.ErrTaskFinished) {
		return errorx.NewCodeError(errorx.CodeTaskFinished, "任务已领完")
	}
	return mapTaskStateErr(err)
}

// mapApplyErr 进度事件错误映射：Invalid→3054、冲突→3053
func mapApplyErr(err error) error {
	if errors.Is(err, task.ErrInvalidTask) {
		return errorx.NewCodeError(errorx.CodeApplyProgressInvalid, "进度事件非法")
	}
	return mapTaskStateErr(err)
}

// mapUnlockErr 通行证解锁错误映射：Invalid→3048、冲突→3053
func mapUnlockErr(err error) error {
	if errors.Is(err, task.ErrInvalidTask) {
		return errorx.NewCodeError(errorx.CodeTaskInvalid, "通行证解锁参数无效")
	}
	return mapTaskStateErr(err)
}

// mapTaskStateErr 乐观锁冲突串 → 3053（其余原样）
func mapTaskStateErr(err error) error {
	if err != nil && (err.Error() == "task claim state conflict" || err.Error() == "task sign-in state conflict" || err.Error() == "membership state conflict") {
		return errorx.NewCodeError(errorx.CodeTaskStateConflict, "任务状态冲突（并发修改）")
	}
	return err
}
