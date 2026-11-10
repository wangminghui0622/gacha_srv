package logic

import (
	"context"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type FindUserPhoneExactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFindUserPhoneExactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FindUserPhoneExactLogic {
	return &FindUserPhoneExactLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// FindUserPhoneExact 精确查绑定用户（未绑定 → 404）
func (l *FindUserPhoneExactLogic) FindUserPhoneExact(in *identity.FindUserPhoneExactReq) (*identity.FindUserPhoneExactResp, error) {
	if in.TenantId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenant required")
	}
	userID, err := l.svcCtx.PhoneRepo.FindExact(l.ctx, in.TenantId, in.Phone)
	if err != nil {
		return nil, mapPhoneRepoError(err)
	}
	return &identity.FindUserPhoneExactResp{UserId: userID}, nil
}
