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

type ClaimPassRewardLogic struct {	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClaimPassRewardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimPassRewardLogic {
	return &ClaimPassRewardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ClaimPassReward 通行证奖励领取（POST /tasks/passes/:passId/claim；requestId 幂等重放）
func (l *ClaimPassRewardLogic) ClaimPassReward(req *types.ClaimPassPathReq, body *types.ClaimPassBodyReq) (resp *types.ClaimPassResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.TaskRpc.ClaimPassReward(l.ctx, &taskClient.ClaimPassRewardReq{
		UserId:    userId,
		PassId:    req.PassId,
		Level:     uint32(body.Level),
		Track:     task.PassTrack(body.Track),
		RequestId: body.RequestId,
	})
	if err != nil {
		l.Errorf("ClaimPassReward failed, userId: %d, passId: %d, err: %v", userId, req.PassId, err)
		return nil, errorx.FromError(err)
	}
	return &types.ClaimPassResp{Level: int64(r.Level), Track: int64(r.Track), RewardTemplateId: r.RewardTemplateId, RewardCount: int64(r.RewardCount)}, nil
}
