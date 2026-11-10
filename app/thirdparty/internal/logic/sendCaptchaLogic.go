package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendCaptchaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendCaptchaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendCaptchaLogic {
	return &SendCaptchaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SendCaptchaLogic) SendCaptcha(in *thirdparty.SendCaptchaReq) (*thirdparty.SendCaptchaResp, error) {
	// 场景守卫照抄 server_v2：UNSPECIFIED 按 LOGIN 处理，其余拒绝
	if in.Scene != thirdparty.CaptchaScene_CAPTCHA_SCENE_LOGIN && in.Scene != thirdparty.CaptchaScene_CAPTCHA_SCENE_UNSPECIFIED {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "unsupported captcha scene")
	}
	expiresAt, err := l.svcCtx.Captcha.SendLoginCaptcha(l.ctx, in.TenantId, in.Phone)
	if err != nil {
		return nil, mapSmsError(err)
	}
	return &thirdparty.SendCaptchaResp{ExpiresAtUnix: expiresAt.Unix()}, nil
}
