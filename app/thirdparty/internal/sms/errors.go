package sms

import "errors"

// 错误哨兵照抄 server_v2 integration/sms（logic 层映射到 errorx 6000 段业务码）
var (
	ErrInvalidPhone   = errors.New("invalid phone number")
	ErrCaptchaBusy    = errors.New("captcha send too frequent")
	ErrCaptchaInvalid = errors.New("invalid captcha")
	ErrSmsDisabled    = errors.New("sms provider not configured")
)
