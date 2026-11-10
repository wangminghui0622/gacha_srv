package logic

import (
	"context"

	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"

	"github.com/zeromicro/go-zero/core/logx"
)

type RequestPhoneLoginCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRequestPhoneLoginCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestPhoneLoginCodeLogic {
	return &RequestPhoneLoginCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ── M15a 手机号/微信登录（请求形状照抄 server_v2，响应维持本仓 LoginResp）──
// RequestPhoneLoginCode 委托第三方服务发登录验证码（scene=LOGIN；
// 下游 6002 频控/6003 未启用/6004 手机号非法经 ClientErrorInterceptor 原样还原）
func (l *RequestPhoneLoginCodeLogic) RequestPhoneLoginCode(in *identity.RequestPhoneLoginCodeReq) (*identity.RequestPhoneLoginCodeResp, error) {
	reply, err := l.svcCtx.ThirdpartyRpc.SendCaptcha(l.ctx, &thirdparty.SendCaptchaReq{
		TenantId: in.TenantId,
		Phone:    in.Phone,
		Scene:    thirdparty.CaptchaScene_CAPTCHA_SCENE_LOGIN,
	})
	if err != nil {
		return nil, err
	}
	return &identity.RequestPhoneLoginCodeResp{ExpiresAtUnix: reply.ExpiresAtUnix}, nil
}
