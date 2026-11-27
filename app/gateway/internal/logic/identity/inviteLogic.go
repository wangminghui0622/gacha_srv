package identity

import (
	"context"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	identityClient "gacha_srv/app/identity/identityClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InviteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewInviteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InviteLogic {
	return &InviteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *InviteLogic) currentUserID() (int64, error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return 0, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	return userId, nil
}

// Info 我的邀请码、邀请人数、已发放/待领取吉祥值。
func (l *InviteLogic) Info() (*types.InviteInfoResp, error) {
	userId, err := l.currentUserID()
	if err != nil {
		return nil, err
	}
	r, err := l.svcCtx.IdentityRpc.GetInviteInfo(l.ctx, &identityClient.GetInviteInfoReq{UserId: userId})
	if err != nil {
		l.Errorf("InviteInfo failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	return &types.InviteInfoResp{
		Code:          r.GetCode(),
		InvitedCount:  r.GetInvitedCount(),
		RewardPoints:  r.GetRewardPoints(),
		PendingPoints: r.GetPendingPoints(),
	}, nil
}

// Bind 绑定邀请码。
func (l *InviteLogic) Bind(code string) (*types.BindInviteCodeResp, error) {
	userId, err := l.currentUserID()
	if err != nil {
		return nil, err
	}
	r, err := l.svcCtx.IdentityRpc.BindInviteCode(l.ctx, &identityClient.BindInviteCodeReq{
		UserId: userId,
		Code:   code,
	})
	if err != nil {
		l.Errorf("BindInviteCode failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	return &types.BindInviteCodeResp{Bound: r.GetBound(), AlreadyBound: r.GetAlreadyBound()}, nil
}

// Records 我的邀请记录。
func (l *InviteLogic) Records() (*types.InviteRecordsResp, error) {
	userId, err := l.currentUserID()
	if err != nil {
		return nil, err
	}
	r, err := l.svcCtx.IdentityRpc.ListInviteRecords(l.ctx, &identityClient.ListInviteRecordsReq{UserId: userId})
	if err != nil {
		l.Errorf("ListInviteRecords failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	items := make([]types.InviteRecordItem, 0, len(r.GetItems()))
	for _, item := range r.GetItems() {
		items = append(items, types.InviteRecordItem{
			InviteeUserId:   item.GetInviteeUserId(),
			InviteePublicId: item.GetInviteePublicId(),
			InviteeNickname: item.GetInviteeNickname(),
			RewardPoints:    item.GetRewardPoints(),
			Status:          item.GetStatus(),
			BoundAtUnix:     item.GetBoundAtUnix(),
			RewardAtUnix:    item.GetRewardAtUnix(),
			ExpiresAtUnix:   item.GetExpiresAtUnix(),
		})
	}
	return &types.InviteRecordsResp{List: items}, nil
}
