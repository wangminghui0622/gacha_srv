package logic

import (
	"context"

	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginWithPhoneCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginWithPhoneCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginWithPhoneCodeLogic {
	return &LoginWithPhoneCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LoginWithPhoneCode 验证码登录：先一次性消费验证码（错码/频控原样透出），
// 再精确查绑定手机；未绑定/解密失败统一 1011（server_v2 ErrInvalidPhoneLogin 语义）
func (l *LoginWithPhoneCodeLogic) LoginWithPhoneCode(in *identity.LoginWithPhoneCodeReq) (*identity.LoginResp, error) {
	if _, err := l.svcCtx.ThirdpartyRpc.CheckCaptcha(l.ctx, &thirdparty.CheckCaptchaReq{
		TenantId: in.TenantId,
		Phone:    in.Phone,
		Code:     in.Code,
		Scene:    thirdparty.CaptchaScene_CAPTCHA_SCENE_LOGIN,
	}); err != nil {
		return nil, err
	}
	userID, err := l.svcCtx.PhoneRepo.FindExact(l.ctx, in.TenantId, in.Phone)
	if err != nil || userID == 0 {
		return nil, errorx.NewCodeError(errorx.CodePhoneLoginInvalid, "手机号未绑定或验证失败")
	}
	return issueToken(l.ctx, l.svcCtx, userID)
}
