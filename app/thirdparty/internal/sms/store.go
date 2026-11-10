// RedisCaptchaStore 照抄 server_v2 integration/sms/redis_captcha.go：
// TTL 15min；key luckybox:sms:captcha:{tenant}:{md5(phone)}；
// 频控列表 24h>10 次或 60s 内重发 → ErrCaptchaBusy；Verify 用 Lua GET+比对+DEL 一次性消费。
package sms

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"

	"gacha_srv/app/thirdparty/internal/core"

	"github.com/redis/go-redis/v9"
)

const captchaTTL = 15 * time.Minute

type RedisCaptchaStore struct {
	client redis.UniversalClient
}

func NewRedisCaptchaStore(client redis.UniversalClient) *RedisCaptchaStore {
	return &RedisCaptchaStore{client: client}
}

func (s *RedisCaptchaStore) Send(ctx context.Context, tenantID int64, phone string) (code string, expiresAt time.Time, err error) {
	if err = s.checkRate(ctx, phone); err != nil {
		return "", time.Time{}, err
	}
	code, err = core.RandomDigits(6)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = time.Now().UTC().Add(captchaTTL)
	key := captchaKey(tenantID, phone)
	if err = s.client.Set(ctx, key, code, captchaTTL).Err(); err != nil {
		return "", time.Time{}, err
	}
	freqKey := frequencyKey(phone)
	if n, err := s.client.RPush(ctx, freqKey, time.Now().Unix()).Result(); err != nil {
		return "", time.Time{}, err
	} else if n == 1 || n > 5 {
		_ = s.client.Expire(ctx, freqKey, 24*time.Hour).Err()
	}
	return code, expiresAt, nil
}

func (s *RedisCaptchaStore) Verify(ctx context.Context, tenantID int64, phone, code string) (bool, error) {
	key := captchaKey(tenantID, phone)
	script := redis.NewScript(`
local v = redis.call("GET", KEYS[1])
if not v then return 0 end
if v ~= ARGV[1] then return 0 end
redis.call("DEL", KEYS[1])
return 1
`)
	res, err := script.Run(ctx, s.client, []string{key}, code).Int64()
	return res == 1, err
}

func (s *RedisCaptchaStore) checkRate(ctx context.Context, phone string) error {
	freqKey := frequencyKey(phone)
	count, err := s.client.LLen(ctx, freqKey).Result()
	if err != nil {
		return err
	}
	if count > 10 {
		return ErrCaptchaBusy
	}
	if count > 0 {
		last, err := s.client.LIndex(ctx, freqKey, -1).Int64()
		if err != nil {
			return err
		}
		if time.Now().Unix()-last < 60 {
			return ErrCaptchaBusy
		}
	}
	return nil
}

func captchaKey(tenantID int64, phone string) string {
	sum := md5.Sum([]byte(phone))
	return fmt.Sprintf("luckybox:sms:captcha:%d:%s", tenantID, hex.EncodeToString(sum[:]))
}

func frequencyKey(phone string) string {
	sum := md5.Sum([]byte(phone))
	return fmt.Sprintf("luckybox:sms:captcha:frequency:%s", hex.EncodeToString(sum[:]))
}
