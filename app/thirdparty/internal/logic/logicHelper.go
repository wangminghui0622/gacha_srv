package logic

import (
	"errors"

	"gacha_srv/app/thirdparty/internal/alipay"
	"gacha_srv/app/thirdparty/internal/sms"
	"gacha_srv/app/thirdparty/internal/wechat"
	"gacha_srv/pkg/utils/errorx"
)

// mapSmsError 短信错误 → errorx 6000 段业务码（05-errors.md 登记）
func mapSmsError(err error) error {
	switch {
	case errors.Is(err, sms.ErrCaptchaBusy):
		return errorx.NewCodeError(errorx.CodeCaptchaBusy, "验证码发送过于频繁")
	case errors.Is(err, sms.ErrCaptchaInvalid):
		return errorx.NewCodeError(errorx.CodeCaptchaInvalid, "验证码无效或已过期")
	case errors.Is(err, sms.ErrInvalidPhone):
		return errorx.NewCodeError(errorx.CodePhoneInvalid, "手机号格式非法")
	case errors.Is(err, sms.ErrSmsDisabled):
		return errorx.NewCodeError(errorx.CodeSmsDisabled, "短信通道未配置")
	default:
		return errorx.NewCodeErrorf(errorx.CodeServerError, "sms: %v", err)
	}
}

// mapWeChatError 微信错误 → errorx 6000 段业务码（05-errors.md 登记）
func mapWeChatError(err error) error {
	switch {
	case errors.Is(err, wechat.ErrInvalidRequest):
		return errorx.NewCodeError(errorx.CodeParamError, err.Error())
	case errors.Is(err, wechat.ErrNotConfigured):
		return errorx.NewCodeError(errorx.CodeWechatNotConfigured, "微信未配置")
	case errors.Is(err, wechat.ErrWeChatAPI):
		return errorx.NewCodeErrorf(errorx.CodeWechatAPIError, "%v", err)
	default:
		return errorx.NewCodeErrorf(errorx.CodeServerError, "wechat: %v", err)
	}
}

// mapAlipayError 支付宝错误 → errorx 6000 段业务码（05-errors.md 登记；
// server_v2 mapThirdPartyError 同款语义：未配置 FailedPrecondition，其余 Internal）。
func mapAlipayError(err error) error {
	if errors.Is(err, alipay.ErrNotConfigured) {
		return errorx.NewCodeError(errorx.CodeAlipayNotConfigured, "支付宝未配置")
	}
	return errorx.NewCodeErrorf(errorx.CodeAlipayAPIError, "%v", err)
}
