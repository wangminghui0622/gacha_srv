package logic

import (
	"context"
	"fmt"
	"time"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/money/moneyClient"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const inviteCodeLength = 12

type GetInviteInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetInviteInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInviteInfoLogic {
	return &GetInviteInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetInviteInfo 我的邀请信息：确保邀请码存在，并结算 10 天内待领取的吉祥值奖励。
func (l *GetInviteInfoLogic) GetInviteInfo(in *identity.GetInviteInfoReq) (*identity.GetInviteInfoResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	code, err := repo.EnsureInviteCode(l.svcCtx.DB, in.UserId, inviteCodeLength)
	if err != nil {
		l.Errorf("GetInviteInfo ensure code failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "邀请码生成失败")
	}

	l.settlePending(in.UserId)

	stats, err := repo.GetInviteStats(l.svcCtx.DB, in.UserId)
	if err != nil {
		l.Errorf("GetInviteInfo stats failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "邀请统计查询失败")
	}
	return &identity.GetInviteInfoResp{
		Code:          code,
		InvitedCount:  stats.InvitedCount,
		RewardPoints:  stats.RewardPoints,
		PendingPoints: stats.PendingPoints,
	}, nil
}

// settlePending 结算邀请人名下待领取奖励：
// 未过期 → CreditWallet(kind=4 吉祥值)，幂等键 invite:<inviteeUserId>；已过期 → 标记过期。
func (l *GetInviteInfoLogic) settlePending(inviterUserID int64) {
	rows, err := repo.ListPendingInvites(l.svcCtx.DB, inviterUserID)
	if err != nil {
		l.Errorf("GetInviteInfo pending failed, userId: %d, err: %v", inviterUserID, err)
		return
	}
	now := time.Now()
	for _, row := range rows {
		if !row.ExpiresAt.After(now) {
			if err := repo.MarkInviteStatus(l.svcCtx.DB, row.InviteeUserID, 2, nil); err != nil {
				l.Errorf("GetInviteInfo expire failed, invitee: %d, err: %v", row.InviteeUserID, err)
			}
			continue
		}
		_, err := l.svcCtx.MoneyRpc.CreditWallet(l.ctx, &moneyClient.CreditWalletReq{
			UserId:    inviterUserID,
			Kind:      4, // LOYALTY_POINT（1 吉祥值 = 1 分存储）
			Amount:    utils.FenToYuan(row.RewardPoints),
			RequestId: fmt.Sprintf("invite:%d", row.InviteeUserID),
		})
		if err != nil {
			l.Errorf("GetInviteInfo credit failed, inviter: %d, invitee: %d, err: %v", inviterUserID, row.InviteeUserID, err)
			continue
		}
		rewardAt := now
		if err := repo.MarkInviteStatus(l.svcCtx.DB, row.InviteeUserID, 1, &rewardAt); err != nil {
			l.Errorf("GetInviteInfo mark reward failed, invitee: %d, err: %v", row.InviteeUserID, err)
		}
	}
}
