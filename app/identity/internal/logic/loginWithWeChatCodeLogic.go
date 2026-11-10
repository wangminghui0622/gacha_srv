package logic

import (
	"context"

	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginWithWeChatCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginWithWeChatCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginWithWeChatCodeLogic {
	return &LoginWithWeChatCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LoginWithWeChatCode 微信登录：code 交换 openid（未配置 6005/接口失败 6006 原样透出）
// → 按 openid 解析用户；未绑定 → 1013
func (l *LoginWithWeChatCodeLogic) LoginWithWeChatCode(in *identity.LoginWithWeChatCodeReq) (*identity.LoginResp, error) {
	exchange, err := l.svcCtx.ThirdpartyRpc.ExchangeWeChatCode(l.ctx, &thirdparty.ExchangeWeChatCodeReq{
		TenantId: in.TenantId,
		Scene:    thirdparty.WeChatOAuthScene(in.Scene),
		Code:     in.Code,
	})
	if err != nil {
		return nil, err
	}
	kind, err := wechatKindFromScene(in.Scene)
	if err != nil {
		return nil, err
	}
	userID, err := l.svcCtx.ThirdPartyRepo.ResolveUser(l.ctx, in.TenantId, kind, exchange.OpenId)
	if err != nil {
		return nil, mapThirdPartyRepoError(err)
	}
	return issueToken(l.ctx, l.svcCtx, userID)
}
