package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckCaptchaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckCaptchaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckCaptchaLogic {
	return &CheckCaptchaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CheckCaptchaLogic) CheckCaptcha(in *thirdparty.CheckCaptchaReq) (*thirdparty.CheckCaptchaResp, error) {
	if in.Scene != thirdparty.CaptchaScene_CAPTCHA_SCENE_LOGIN && in.Scene != thirdparty.CaptchaScene_CAPTCHA_SCENE_UNSPECIFIED {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "unsupported captcha scene")
	}
	if err := l.svcCtx.Captcha.CheckLoginCaptcha(l.ctx, in.TenantId, in.Phone, in.Code); err != nil {
		return nil, mapSmsError(err)
	}
	return &thirdparty.CheckCaptchaResp{}, nil
}
