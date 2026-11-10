package logic

import (
	"context"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetThirdPartyIdentityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetThirdPartyIdentityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetThirdPartyIdentityLogic {
	return &GetThirdPartyIdentityLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetThirdPartyIdentity 查用户某类第三方身份（未绑定 → 1013）
func (l *GetThirdPartyIdentityLogic) GetThirdPartyIdentity(in *identity.GetThirdPartyIdentityReq) (*identity.GetThirdPartyIdentityResp, error) {
	if in.TenantId == 0 || in.UserId == 0 || !validThirdPartyKind(in.Kind) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "非法查询请求")
	}
	row, err := l.svcCtx.ThirdPartyRepo.GetByUser(l.ctx, in.TenantId, in.UserId, int64(in.Kind))
	if err != nil {
		return nil, mapThirdPartyRepoError(err)
	}
	return &identity.GetThirdPartyIdentityResp{OpenId: row.OpenId, UnionId: row.UnionId}, nil
}
