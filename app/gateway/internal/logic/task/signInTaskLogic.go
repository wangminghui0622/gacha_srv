package task

import (
	"context"

	taskClient "gacha_srv/app/task/taskClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SignInTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSignInTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignInTaskLogic {
	return &SignInTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SignInTask 每日签到（POST /tasks/:taskId/sign-in；老项目 BindUser 语义：用户身份以令牌为准）
func (l *SignInTaskLogic) SignInTask(req *types.TaskPathReq, body *types.SignInBodyReq) (resp *types.SignInResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.TaskRpc.SignInTask(l.ctx, &taskClient.SignInTaskReq{
		UserId:  userId,
		TaskId:  req.TaskId,
		Version: body.Version,
	})
	if err != nil {
		l.Errorf("SignInTask failed, userId: %d, taskId: %d, err: %v", userId, req.TaskId, err)
		return nil, errorx.FromError(err)
	}
	return &types.SignInResp{Streak: int64(r.Streak), CalendarDate: r.CalendarDate, AlreadySigned: r.AlreadySigned}, nil
}
