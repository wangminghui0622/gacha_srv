package core

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// 会话语义照抄 server_v2 identity biz/session.go：
// access = EdDSA JWT（jti=会话 ID，roles 为角色编码快照，服务端权威解析）；
// refresh = 随机令牌，库中只存 SHA256；旋转 + 指纹校验 + 家族吊销。

var (
	// ErrInvalidCredentials 用户名或密码错误。
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidRefreshToken refresh 令牌无效（含指纹不符/家族泄露）。
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// ConsoleAccount 控制台账号（会话视角）。
type ConsoleAccount struct {
	ID           uint64
	TenantID     uint64
	Username     string
	DisplayName  string
	PasswordHash string
	RoleCodes    []string
	Disabled     bool
	TotpSecret   string
}

// ConsoleSession 控制台会话（存储视角）。
type ConsoleSession struct {
	Jti                 string
	FamilyID            string
	AccountID           uint64
	TenantID            uint64
	DisplayName         string
	RoleCodes           []string
	RefreshHash         string // hex(SHA256(refresh))
	PreviousRefreshHash string // hex(SHA256(上一枚 refresh))，重放检测
	FingerprintHash     string // hex(SHA256(fingerprint))
	ExpiresAt           time.Time
	RevokedAt           *time.Time
	Compromised         bool
}

// ConsoleAccountStore 账号读写接口（repo 层用 GORM 模型实现）。
type ConsoleAccountStore interface {
	FindByUsername(ctx context.Context, tenantID uint64, username string) (*ConsoleAccount, error)
	FindByID(ctx context.Context, tenantID, accountID uint64) (*ConsoleAccount, error)
	EnableTotp(ctx context.Context, tenantID, accountID uint64, secret string) error
	ResetTotp(ctx context.Context, tenantID, accountID uint64) error
}

// ConsoleSessionStore 会话读写接口。
type ConsoleSessionStore interface {
	Create(ctx context.Context, s *ConsoleSession) error
	// FindByRefreshHash 按 refresh 哈希查会话（含上一枚哈希；matchedViaPrevious=true
	// 表示重放已旋转的旧令牌 → 调用方吊销整族）。
	FindByRefreshHash(ctx context.Context, hash string) (session *ConsoleSession, matchedViaPrevious bool, err error)
	// Rotate 原子旋转 refresh 哈希（WHERE jti AND refresh_hash=old 守卫，滑动续期，
	// 旧哈希存 previous 供重放检测）
	Rotate(ctx context.Context, jti, oldHash, newHash string, newExpiresAt time.Time) error
	Revoke(ctx context.Context, jti string, compromised bool) error
	RevokeActiveByAccount(ctx context.Context, tenantID, accountID uint64) ([]string, error)
}

// SessionRevocationPublisher 会话吊销发布（Redis 键，网关本地验签时查）。
type SessionRevocationPublisher interface {
	MarkRevoked(ctx context.Context, sessionID string, ttl time.Duration) error
}

// SessionManager 控制台会话管理器。
type SessionManager struct {
	accounts    ConsoleAccountStore
	sessions    ConsoleSessionStore
	hasher      Argon2idHasher
	privateKey  ed25519.PrivateKey
	issuer      string
	audience    string
	accessTTL   time.Duration
	refreshTTL  time.Duration
	revocations SessionRevocationPublisher
	now         func() time.Time
}

// NewSessionManager 构造会话管理器（TTL 校验同 server_v2）。
func NewSessionManager(accounts ConsoleAccountStore, sessions ConsoleSessionStore, hasher Argon2idHasher,
	privateKey ed25519.PrivateKey, issuer, audience string, accessTTL, refreshTTL time.Duration) (*SessionManager, error) {
	if accounts == nil || sessions == nil || len(privateKey) != ed25519.PrivateKeySize || issuer == "" || audience == "" ||
		accessTTL <= 0 || refreshTTL <= accessTTL {
		return nil, errors.New("会话管理器配置无效")
	}
	return &SessionManager{accounts: accounts, sessions: sessions, hasher: hasher, privateKey: privateKey,
		issuer: issuer, audience: audience, accessTTL: accessTTL, refreshTTL: refreshTTL, now: time.Now}, nil
}

// WithRevocationPublisher 注入 Redis 吊销发布器。
func (m *SessionManager) WithRevocationPublisher(p SessionRevocationPublisher) *SessionManager {
	m.revocations = p
	return m
}

// VerifyPassword 校验口令（供登录流程使用）。
func (m *SessionManager) VerifyPassword(hash, password string) bool {
	return m.hasher.Verify(hash, password)
}

// CreateSessionForAccount 为账号创建新会话（登录/TOTP 完成后调用）。
func (m *SessionManager) CreateSessionForAccount(ctx context.Context, account *ConsoleAccount, fingerprint string) (*TokenPair, error) {
	if account == nil || fingerprint == "" {
		return nil, ErrInvalidCredentials
	}
	if account.Disabled {
		return nil, ErrInvalidCredentials
	}
	now := m.now()
	sessionID, err := randomToken(24)
	if err != nil {
		return nil, err
	}
	refresh, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	session := &ConsoleSession{Jti: sessionID, FamilyID: sessionID, AccountID: account.ID, TenantID: account.TenantID,
		DisplayName: account.DisplayName,
		RoleCodes:   append([]string(nil), account.RoleCodes...),
		RefreshHash: hashHex(refresh), FingerprintHash: hashHex(fingerprint), ExpiresAt: now.Add(m.refreshTTL)}
	if err = m.sessions.Create(ctx, session); err != nil {
		return nil, err
	}
	return m.issue(session, refresh, now)
}

// Refresh 旋转 refresh 令牌（指纹不符/已吊销/过期 → 整族吊销；
// 重放已旋转的旧令牌（previous 命中）→ 整族吊销）。
func (m *SessionManager) Refresh(ctx context.Context, refresh, fingerprint string) (*TokenPair, error) {
	if refresh == "" || fingerprint == "" {
		return nil, ErrInvalidRefreshToken
	}
	session, matchedViaPrevious, err := m.sessions.FindByRefreshHash(ctx, hashHex(refresh))
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}
	now := m.now()
	if matchedViaPrevious || session.RevokedAt != nil || session.Compromised ||
		!now.Before(session.ExpiresAt) || session.FingerprintHash != hashHex(fingerprint) {
		m.revoke(ctx, session.FamilyID, true, session.ExpiresAt.Sub(now))
		return nil, ErrInvalidRefreshToken
	}
	next, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	newExpiresAt := now.Add(m.refreshTTL)
	if err = m.sessions.Rotate(ctx, session.Jti, session.RefreshHash, hashHex(next), newExpiresAt); err != nil {
		m.revoke(ctx, session.FamilyID, true, session.ExpiresAt.Sub(now))
		return nil, ErrInvalidRefreshToken
	}
	session.RefreshHash = hashHex(next)
	session.ExpiresAt = newExpiresAt
	// 重签发时重载账号（角色/显示名取最新；JWT 快照仅参考，鉴权仍服务端解析）。
	// 账号已停用/不存在 → 家族吊销。
	account, err := m.accounts.FindByID(ctx, session.TenantID, session.AccountID)
	if err != nil || account.Disabled {
		m.revoke(ctx, session.FamilyID, true, session.ExpiresAt.Sub(now))
		return nil, ErrInvalidRefreshToken
	}
	session.DisplayName = account.DisplayName
	session.RoleCodes = account.RoleCodes
	return m.issue(session, next, now)
}

// RevokeAccountSessions 吊销某账号全部活跃会话（TOTP 重置后调用），返回吊销数。
func (m *SessionManager) RevokeAccountSessions(ctx context.Context, tenantID, accountID uint64) (uint32, error) {
	if tenantID == 0 || accountID == 0 {
		return 0, errors.New("tenant 与 account 必填")
	}
	ids, err := m.sessions.RevokeActiveByAccount(ctx, tenantID, accountID)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if m.revocations != nil {
			_ = m.revocations.MarkRevoked(ctx, id, m.refreshTTL)
		}
	}
	return uint32(len(ids)), nil
}

// Logout 吊销指定会话（session_id = JWT jti，由网关从令牌解析后传入）。
func (m *SessionManager) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return errors.New("会话 ID 必填")
	}
	if m.revocations != nil {
		if err := m.revocations.MarkRevoked(ctx, sessionID, m.refreshTTL); err != nil {
			return err
		}
	}
	return m.sessions.Revoke(ctx, sessionID, false)
}

func (m *SessionManager) revoke(ctx context.Context, sessionID string, compromised bool, ttl time.Duration) {
	_ = m.sessions.Revoke(ctx, sessionID, compromised)
	if m.revocations != nil && ttl > 0 {
		_ = m.revocations.MarkRevoked(ctx, sessionID, ttl)
	}
}

// TokenPair 登录/刷新产物。
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	SessionID        string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

func (m *SessionManager) issue(session *ConsoleSession, refresh string, now time.Time) (*TokenPair, error) {
	accessExpires := now.Add(m.accessTTL)
	claims := jwt.MapClaims{
		"actor_id": session.AccountID, "tenant_id": session.TenantID, "name": session.DisplayName,
		"roles": session.RoleCodes, "iss": m.issuer, "aud": m.audience, "sub": fmt.Sprint(session.AccountID),
		"jti": session.Jti, "iat": now.Unix(), "exp": accessExpires.Unix(),
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(m.privateKey)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, SessionID: session.Jti,
		AccessExpiresAt: accessExpires, RefreshExpiresAt: session.ExpiresAt}, nil
}

func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// RandomToken 生成 base64url 随机串（会话 ID/refresh/登录票据通用）。
func RandomToken(size int) (string, error) {
	return randomToken(size)
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
