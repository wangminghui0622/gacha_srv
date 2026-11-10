// CaptchaService 照抄 server_v2 integration/sms/captcha.go：
// 发送前校验手机号格式 → store 发码（含频控）→ sender 真实/跳过发送；
// 校验 = store 一次性消费。
package sms

import (
	"context"
	"time"

	"gacha_srv/app/thirdparty/internal/core"
)

type CaptchaService struct {
	store  *RedisCaptchaStore
	sender *Sender
}

func NewCaptchaService(store *RedisCaptchaStore, sender *Sender) *CaptchaService {
	return &CaptchaService{store: store, sender: sender}
}

func (s *CaptchaService) SendLoginCaptcha(ctx context.Context, tenantID int64, phone string) (time.Time, error) {
	if tenantID == 0 {
		return time.Time{}, ErrInvalidPhone
	}
	normalized, err := core.NormalizeCNMobile(phone)
	if err != nil {
		return time.Time{}, ErrInvalidPhone
	}
	code, expiresAt, err := s.store.Send(ctx, tenantID, normalized)
	if err != nil {
		return time.Time{}, err
	}
	if err = s.sender.Send(ctx, normalized, code); err != nil {
		return time.Time{}, err
	}
	return expiresAt, nil
}

func (s *CaptchaService) CheckLoginCaptcha(ctx context.Context, tenantID int64, phone, code string) error {
	if tenantID == 0 || code == "" {
		return ErrCaptchaInvalid
	}
	normalized, err := core.NormalizeCNMobile(phone)
	if err != nil {
		return ErrInvalidPhone
	}
	ok, err := s.store.Verify(ctx, tenantID, normalized, code)
	if err != nil || !ok {
		return ErrCaptchaInvalid
	}
	return nil
}
