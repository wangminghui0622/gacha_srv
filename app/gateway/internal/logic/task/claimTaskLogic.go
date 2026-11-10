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

type ClaimTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClaimTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimTaskLogic {
	return &ClaimTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ClaimTask 任务奖励领取（POST /tasks/:taskId/claim；requestId 幂等重放）
func (l *ClaimTaskLogic) ClaimTask(req *types.TaskPathReq, body *types.ClaimTaskBodyReq) (resp *types.ClaimTaskResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.TaskRpc.ClaimTask(l.ctx, &taskClient.ClaimTaskReq{
		UserId:    userId,
		TaskId:    req.TaskId,
		Version:   body.Version,
		RequestId: body.RequestId,
	})
	if err != nil {
		l.Errorf("ClaimTask failed, userId: %d, taskId: %d, err: %v", userId, req.TaskId, err)
		return nil, errorx.FromError(err)
	}
	return &types.ClaimTaskResp{RewardTemplateId: r.RewardTemplateId, RewardCount: int64(r.RewardCount), Claimed: int64(r.Claimed)}, nil
}
