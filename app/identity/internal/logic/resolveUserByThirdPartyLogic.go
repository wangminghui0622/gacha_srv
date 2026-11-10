package logic

import (
	"context"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveUserByThirdPartyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveUserByThirdPartyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveUserByThirdPartyLogic {
	return &ResolveUserByThirdPartyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveUserByThirdParty 按 openid 解析用户（未绑定 → 1013）
func (l *ResolveUserByThirdPartyLogic) ResolveUserByThirdParty(in *identity.ResolveUserByThirdPartyReq) (*identity.ResolveUserByThirdPartyResp, error) {
	if in.TenantId == 0 || in.OpenId == "" || !validThirdPartyKind(in.Kind) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "非法解析请求")
	}
	userID, err := l.svcCtx.ThirdPartyRepo.ResolveUser(l.ctx, in.TenantId, int64(in.Kind), in.OpenId)
	if err != nil {
		return nil, mapThirdPartyRepoError(err)
	}
	return &identity.ResolveUserByThirdPartyResp{UserId: userID}, nil
}
