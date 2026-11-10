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

type ClaimPassRewardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimPassRewardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimPassRewardLogic {
	return &ClaimPassRewardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ClaimPassReward 通行证奖励领取（老项目 ClaimPass）：request_id 幂等重放；
// 等级未达 → 3049；槽位已领 → 3050；PREMIUM 线按 PAYMENT(已解锁)/MEMBER(会员等级) 校验。
// 奖励仅记录不发放（D3）。
func (l *ClaimPassRewardLogic) ClaimPassReward(in *taskpb.ClaimPassRewardReq) (*taskpb.ClaimPassRewardResp, error) {
	if in.UserId <= 0 || in.PassId <= 0 || in.Level == 0 || in.RequestId == "" || len(in.RequestId) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeTaskInvalid, "任务参数无效")
	}
	snapshot, err := loadTaskSnapshot(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	season, ok := snapshot.Passes[uint64(in.PassId)]
	if !ok {
		return nil, errorx.NewCodeError(errorx.CodeNotFound, "通行证赛季不存在")
	}
	var reward task.PassReward
	found := false
	for _, item := range season.Rewards {
		if item.Level == in.Level && item.Track == in.Track {
			reward = item
			found = true
			break
		}
	}
	if !found {
		return nil, errorx.NewCodeError(errorx.CodeNotFound, "通行证奖励不存在")
	}
	var claim task.PassClaim
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		claim, err = repo.ClaimPassReward(l.ctx, tx, task.PassClaimCommand{
			RequestID: in.RequestId, UserID: uint64(in.UserId), Season: season, Reward: reward,
		})
		return err
	})
	if err != nil {
		return nil, mapPassErr(err)
	}
	return &taskpb.ClaimPassRewardResp{Level: claim.Level, Track: claim.Track, RewardTemplateId: claim.RewardTemplateID, RewardCount: claim.RewardCount}, nil
}
