package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/ops/internal/core"

	"github.com/redis/go-redis/v9"
)

const (
	// loginTicketTTL 登录票据有效期（照抄 server_v2：5 分钟）。
	loginTicketTTL = 5 * time.Minute
	// loginTicketPrefix Redis 键前缀。
	loginTicketPrefix = "console:login_ticket:"
	// revokedSessionPrefix 会话吊销键前缀（网关本地验签时查此键）。
	revokedSessionPrefix = "console:session:revoked:"
)

// LoginTicketPayload 登录票据内容（TOTP 两步登录的中间态）。
type LoginTicketPayload struct {
	TenantID  uint64 `json:"tenant_id"`
	AccountID uint64 `json:"account_id"`
	Username  string `json:"username"`
	// PendingTotpSecret setup 流程待绑定的密钥（verify 流程为空）
	PendingTotpSecret string `json:"pending_totp_secret,omitempty"`
}

// LoginTicketStore Redis 登录票据（core.ConsoleLoginTicketStore 语义）。
type LoginTicketStore struct {
	rdb *redis.Client
}

// NewLoginTicketStore 构造票据存储。
func NewLoginTicketStore(rdb *redis.Client) *LoginTicketStore {
	return &LoginTicketStore{rdb: rdb}
}

// Save 写入票据（5 分钟 TTL）。
func (s *LoginTicketStore) Save(ctx context.Context, ticket string, payload LoginTicketPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, loginTicketPrefix+ticket, data, loginTicketTTL).Err()
}

// Get 读取票据；不存在返回 nil。
func (s *LoginTicketStore) Get(ctx context.Context, ticket string) (*LoginTicketPayload, error) {
	data, err := s.rdb.Get(ctx, loginTicketPrefix+ticket).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var payload LoginTicketPayload
	if err = json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// Delete 删除票据（登录完成后一次性消费）。
func (s *LoginTicketStore) Delete(ctx context.Context, ticket string) error {
	return s.rdb.Del(ctx, loginTicketPrefix+ticket).Err()
}

// RevocationPublisher Redis 会话吊销发布器（core.SessionRevocationPublisher 实现）。
// 键 console:session:revoked:<jti>，TTL=refresh 有效期；管理网关每请求查此键。
type RevocationPublisher struct {
	rdb *redis.Client
}

// NewRevocationPublisher 构造吊销发布器。
func NewRevocationPublisher(rdb *redis.Client) *RevocationPublisher {
	return &RevocationPublisher{rdb: rdb}
}

// MarkRevoked 标记会话吊销。
func (p *RevocationPublisher) MarkRevoked(ctx context.Context, sessionID string, ttl time.Duration) error {
	return p.rdb.Set(ctx, revokedSessionPrefix+sessionID, "1", ttl).Err()
}

// IsRevoked 查询会话是否已吊销（管理网关用；不存在=未吊销）。
func IsRevoked(ctx context.Context, rdb *redis.Client, sessionID string) (bool, error) {
	err := rdb.Get(ctx, revokedSessionPrefix+sessionID).Err()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("查询会话吊销状态: %w", err)
	}
	return true, nil
}

// 确保实现核心接口（编译期校验）。
var _ core.ConsoleAccountStore = (*AccountStore)(nil)
var _ core.ConsoleSessionStore = (*SessionStore)(nil)
var _ core.SessionRevocationPublisher = (*RevocationPublisher)(nil)
