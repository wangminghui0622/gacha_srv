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

type SignInTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSignInTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignInTaskLogic {
	return &SignInTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SignInTask 每日签到（老项目 SignIn）：同上海日历日重复签到返回当前 streak + already=true；
// 连续昨日 streak++，断签重置 1。任务不存在或非 SIGN_IN → 404。
func (l *SignInTaskLogic) SignInTask(in *taskpb.SignInTaskReq) (*taskpb.SignInTaskResp, error) {
	if in.UserId <= 0 || in.TaskId <= 0 || in.Version <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskInvalid, "任务参数无效")
	}
	snapshot, err := loadTaskSnapshot(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	def, ok := findSignInDef(snapshot, uint64(in.TaskId), uint64(in.Version))
	if !ok {
		return nil, errorx.NewCodeError(errorx.CodeNotFound, "任务不存在")
	}
	now := taskNow()
	today := now.Format("2006-01-02")
	var result task.SignInResult
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = repo.SignInUser(l.ctx, tx, task.SignInCommand{
			UserID: uint64(in.UserId), TaskID: def.ID, Today: today, Previous: task.PreviousDate(today, taskLocation),
		})
		return err
	})
	if err != nil {
		return nil, mapSignInErr(err)
	}
	return &taskpb.SignInTaskResp{Streak: result.Streak, CalendarDate: result.Date, AlreadySigned: result.Already}, nil
}
