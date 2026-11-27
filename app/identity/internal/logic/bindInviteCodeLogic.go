package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindInviteCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindInviteCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindInviteCodeLogic {
	return &BindInviteCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BindInviteCode 绑定邀请关系：被邀请人只能绑定一次；奖励由邀请人打开邀请页时结算。
func (l *BindInviteCodeLogic) BindInviteCode(in *identity.BindInviteCodeReq) (*identity.BindInviteCodeResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	code := strings.TrimSpace(in.Code)
	if len(code) < 4 || len(code) > 32 {
		return nil, errorx.NewCodeError(errorx.CodeInviteCodeInvalid, "邀请码无效")
	}

	inviterID, err := repo.FindInviterByCode(l.svcCtx.DB, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.NewCodeError(errorx.CodeInviteCodeInvalid, "邀请码无效")
		}
		l.Errorf("BindInviteCode find inviter failed, code: %s, err: %v", code, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "邀请码查询失败")
	}
	if inviterID == in.UserId {
		return nil, errorx.NewCodeError(errorx.CodeInviteSelf, "不能邀请自己")
	}
	// 防止 A 邀请 B 后 B 又反向邀请 A 形成环
	if inviterRecord, err := repo.GetInviteRecord(l.svcCtx.DB, inviterID); err == nil &&
		inviterRecord != nil && inviterRecord.InviterUserID == in.UserId {
		return nil, errorx.NewCodeError(errorx.CodeInviteSelf, "不能相互邀请")
	}

	existing, err := repo.GetInviteRecord(l.svcCtx.DB, in.UserId)
	if err != nil {
		l.Errorf("BindInviteCode get record failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "邀请关系查询失败")
	}
	if existing != nil {
		if existing.InviterUserID == inviterID {
			return &identity.BindInviteCodeResp{Bound: true, AlreadyBound: true}, nil
		}
		return nil, errorx.NewCodeError(errorx.CodeInviteAlreadyBound, "该用户已绑定其他邀请人")
	}

	now := time.Now()
	record := repo.InviteRecordRow{
		InviteeUserID: in.UserId,
		InviterUserID: inviterID,
		InviteCode:    code,
		RewardPoints:  inviteRewardPoints(l.svcCtx.Config),
		Status:        0,
		BoundAt:       now,
		ExpiresAt:     now.AddDate(0, 0, int(inviteRewardValidDays(l.svcCtx.Config))),
	}
	if err := repo.CreateInviteRecord(l.svcCtx.DB, record); err != nil {
		// 并发重复绑定：以库里已有关系为准
		if current, getErr := repo.GetInviteRecord(l.svcCtx.DB, in.UserId); getErr == nil && current != nil {
			if current.InviterUserID == inviterID {
				return &identity.BindInviteCodeResp{Bound: true, AlreadyBound: true}, nil
			}
			return nil, errorx.NewCodeError(errorx.CodeInviteAlreadyBound, "该用户已绑定其他邀请人")
		}
		l.Errorf("BindInviteCode create failed, userId: %d, code: %s, err: %v", in.UserId, code, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "邀请关系绑定失败")
	}
	return &identity.BindInviteCodeResp{Bound: true}, nil
}
