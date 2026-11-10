package logic

import (
	"context"
	"time"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type LinkThirdPartyIdentityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLinkThirdPartyIdentityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LinkThirdPartyIdentityLogic {
	return &LinkThirdPartyIdentityLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ── M15a 第三方身份（Link 为内部 RPC，无网关路由；登录靠 Resolve）──
// LinkThirdPartyIdentity 绑定第三方身份（upsert：同 openid 冲突改绑 user/union，server_v2 语义）
func (l *LinkThirdPartyIdentityLogic) LinkThirdPartyIdentity(in *identity.LinkThirdPartyIdentityReq) (*identity.LinkThirdPartyIdentityResp, error) {
	if in.TenantId == 0 || in.UserId == 0 || in.OpenId == "" || !validThirdPartyKind(in.Kind) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "非法绑定请求")
	}
	unionID := in.UnionId
	if unionID == "" {
		unionID = in.OpenId // 与 server_v2 Link 一致：union 缺省即 openid
	}
	err := l.svcCtx.ThirdPartyRepo.Link(l.ctx, &gacha_identity.UserThirdParties{
		TenantId:  in.TenantId,
		UserId:    in.UserId,
		Kind:      int64(in.Kind),
		OpenId:    in.OpenId,
		UnionId:   unionID,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return &identity.LinkThirdPartyIdentityResp{}, nil
}
