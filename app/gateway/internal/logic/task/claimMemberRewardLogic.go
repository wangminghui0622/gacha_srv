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

type ClaimMemberRewardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClaimMemberRewardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimMemberRewardLogic {
	return &ClaimMemberRewardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ClaimMemberReward 会员奖励领取（POST /membership/claim；requestId 幂等重放）
func (l *ClaimMemberRewardLogic) ClaimMemberReward(body *types.ClaimMemberBodyReq) (resp *types.ClaimMemberResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.TaskRpc.ClaimMemberReward(l.ctx, &taskClient.ClaimMemberRewardReq{
		UserId:    userId,
		Level:     uint32(body.Level),
		RequestId: body.RequestId,
	})
	if err != nil {
		l.Errorf("ClaimMemberReward failed, userId: %d, level: %d, err: %v", userId, body.Level, err)
		return nil, errorx.FromError(err)
	}
	return &types.ClaimMemberResp{RewardTemplateId: r.RewardTemplateId, RewardCount: int64(r.RewardCount), Claimed: int64(r.Claimed)}, nil
}
