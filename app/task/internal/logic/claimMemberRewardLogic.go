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

type ClaimMemberRewardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimMemberRewardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimMemberRewardLogic {
	return &ClaimMemberRewardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ClaimMemberReward 会员奖励领取（老项目 ClaimMember）：request_id 幂等重放；
// 消费未达档 → 3051；档已领 → 3052。奖励仅记录不发放（D3）。
func (l *ClaimMemberRewardLogic) ClaimMemberReward(in *taskpb.ClaimMemberRewardReq) (*taskpb.ClaimMemberRewardResp, error) {
	if in.UserId <= 0 || in.Level == 0 || in.RequestId == "" || len(in.RequestId) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeTaskInvalid, "任务参数无效")
	}
	snapshot, err := loadTaskSnapshot(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	var tier task.MemberTier
	found := false
	for _, item := range snapshot.MemberTiers {
		if item.Level == in.Level {
			tier = item
			found = true
			break
		}
	}
	if !found {
		return nil, errorx.NewCodeError(errorx.CodeNotFound, "会员档位不存在")
	}
	var claim task.Claim
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		claim, err = repo.ClaimMemberReward(l.ctx, tx, task.MemberClaimCommand{
			RequestID: in.RequestId, UserID: uint64(in.UserId), Tier: tier,
		})
		return err
	})
	if err != nil {
		return nil, mapMemberErr(err)
	}
	return &taskpb.ClaimMemberRewardResp{RewardTemplateId: claim.RewardTemplateID, RewardCount: claim.RewardCount, Claimed: claim.Claimed}, nil
}
