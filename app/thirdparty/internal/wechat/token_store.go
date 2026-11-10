// TokenStore 微信小程序 access_token Redis 存储（server_v2 token_store.go 逐字同款）：
// key luckybox:integration:wechat:mini:access_token，TTL 传 expires_in 减 60s 余量。
package wechat

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const miniAccessTokenKey = "luckybox:integration:wechat:mini:access_token"

type TokenStore struct {
	client redis.UniversalClient
}

func NewTokenStore(client redis.UniversalClient) *TokenStore {
	return &TokenStore{client: client}
}

// GetMiniAccessToken 读缓存 token（未命中返回空串，非错误）。
func (s *TokenStore) GetMiniAccessToken(ctx context.Context) (string, error) {
	val, err := s.client.Get(ctx, miniAccessTokenKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// SetMiniAccessToken 写缓存 token（ttl<=0 兜底 2h）。
func (s *TokenStore) SetMiniAccessToken(ctx context.Context, token string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	return s.client.Set(ctx, miniAccessTokenKey, token, ttl).Err()
}

// DeleteMiniAccessToken 清缓存 token（40001/42001 触发）。
func (s *TokenStore) DeleteMiniAccessToken(ctx context.Context) error {
	return s.client.Del(ctx, miniAccessTokenKey).Err()
}
