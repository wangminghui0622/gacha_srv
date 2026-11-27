package logic

import (
	"context"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListInviteRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInviteRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInviteRecordsLogic {
	return &ListInviteRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListInviteRecords 邀请人视角的邀请记录。
func (l *ListInviteRecordsLogic) ListInviteRecords(in *identity.ListInviteRecordsReq) (*identity.ListInviteRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	rows, err := repo.ListInviteRecords(l.svcCtx.DB, in.UserId)
	if err != nil {
		l.Errorf("ListInviteRecords failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "邀请记录查询失败")
	}
	items := make([]*identity.InviteRecordItem, 0, len(rows))
	for _, row := range rows {
		item := &identity.InviteRecordItem{
			InviteeUserId:   row.InviteeUserID,
			InviteePublicId: row.PublicID,
			InviteeNickname: row.Nickname,
			RewardPoints:    row.RewardPoints,
			Status:          row.Status,
			BoundAtUnix:     row.BoundAt.Unix(),
			ExpiresAtUnix:   row.ExpiresAt.Unix(),
			RewardAtUnix:    0,
		}
		if row.RewardAt != nil {
			item.RewardAtUnix = row.RewardAt.Unix()
		}
		items = append(items, item)
	}
	return &identity.ListInviteRecordsResp{Items: items}, nil
}
