package core

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// ── 内存假存储（会话管理单测）──

type memAccountStore struct {
	mu       sync.Mutex
	accounts map[string]*ConsoleAccount
}

func newMemAccounts() *memAccountStore {
	return &memAccountStore{accounts: map[string]*ConsoleAccount{}}
}

func (s *memAccountStore) put(a *ConsoleAccount) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[a.Username] = a
}

func (s *memAccountStore) FindByUsername(ctx context.Context, tenantID uint64, username string) (*ConsoleAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[username]
	if !ok || a.TenantID != tenantID {
		return nil, ErrInvalidCredentials
	}
	return cloneAccount(a), nil
}

func (s *memAccountStore) FindByID(ctx context.Context, tenantID, accountID uint64) (*ConsoleAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.accounts {
		if a.TenantID == tenantID && a.ID == accountID {
			return cloneAccount(a), nil
		}
	}
	return nil, ErrInvalidCredentials
}

func (s *memAccountStore) EnableTotp(ctx context.Context, tenantID, accountID uint64, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.accounts {
		if a.TenantID == tenantID && a.ID == accountID {
			a.TotpSecret = secret
			return nil
		}
	}
	return ErrInvalidCredentials
}

func (s *memAccountStore) ResetTotp(ctx context.Context, tenantID, accountID uint64) error {
	return s.EnableTotp(ctx, tenantID, accountID, "")
}

func cloneAccount(a *ConsoleAccount) *ConsoleAccount {
	c := *a
	c.RoleCodes = append([]string(nil), a.RoleCodes...)
	return &c
}

type memSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*ConsoleSession
}

func newMemSessions() *memSessionStore {
	return &memSessionStore{sessions: map[string]*ConsoleSession{}}
}

func (s *memSessionStore) Create(ctx context.Context, session *ConsoleSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.Jti] = session
	return nil
}

func (s *memSessionStore) FindByRefreshHash(ctx context.Context, hash string) (*ConsoleSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if session.RefreshHash == hash {
			return session, false, nil
		}
	}
	for _, session := range s.sessions {
		if session.PreviousRefreshHash == hash {
			return session, true, nil
		}
	}
	return nil, false, ErrInvalidRefreshToken
}

func (s *memSessionStore) Rotate(ctx context.Context, jti, oldHash, newHash string, newExpiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[jti]
	if session == nil || session.RefreshHash != oldHash {
		return ErrInvalidRefreshToken
	}
	session.PreviousRefreshHash = oldHash
	session.RefreshHash = newHash
	session.ExpiresAt = newExpiresAt
	return nil
}

func (s *memSessionStore) Revoke(ctx context.Context, jti string, compromised bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[jti]
	if session == nil || session.RevokedAt != nil {
		return nil
	}
	now := time.Now()
	session.RevokedAt = &now
	session.Compromised = compromised
	return nil
}

func (s *memSessionStore) RevokeActiveByAccount(ctx context.Context, tenantID, accountID uint64) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	now := time.Now()
	for jti, session := range s.sessions {
		if session.TenantID == tenantID && session.AccountID == accountID && session.RevokedAt == nil {
			session.RevokedAt = &now
			ids = append(ids, jti)
		}
	}
	return ids, nil
}

// ── 测试 ──

func newTestSessionManager(t *testing.T) (*SessionManager, *memAccountStore, ed25519.PublicKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	accounts := newMemAccounts()
	accounts.put(&ConsoleAccount{ID: 7, TenantID: 1, Username: "admin", DisplayName: "超级管理员",
		RoleCodes: []string{"super_admin"}})
	mgr, err := NewSessionManager(accounts, newMemSessions(), DefaultArgon2idHasher(),
		priv, "luckybox-console", "console-api", 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return mgr, accounts, priv.Public().(ed25519.PublicKey)
}

func TestSessionManager_CreateAndVerifyClaims(t *testing.T) {
	mgr, accounts, pub := newTestSessionManager(t)
	account, _ := accounts.FindByID(context.Background(), 1, 7)
	pair, err := mgr.CreateSessionForAccount(context.Background(), account, "fp-123")
	if err != nil {
		t.Fatal(err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" || pair.SessionID == "" {
		t.Fatalf("token pair incomplete: %+v", pair)
	}
	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(pair.AccessToken, claims, func(tok *jwt.Token) (any, error) { return pub, nil })
	if err != nil {
		t.Fatalf("access token should verify with the public key: %v", err)
	}
	if claims["iss"] != "luckybox-console" || claims["aud"] != "console-api" ||
		claims["actor_id"] != float64(7) || claims["tenant_id"] != float64(1) {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestSessionManager_RefreshRotates(t *testing.T) {
	mgr, accounts, pub := newTestSessionManager(t)
	account, _ := accounts.FindByID(context.Background(), 1, 7)
	pair, err := mgr.CreateSessionForAccount(context.Background(), account, "fp-123")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := mgr.Refresh(context.Background(), pair.RefreshToken, "fp-123")
	if err != nil {
		t.Fatal(err)
	}
	if rotated.RefreshToken == pair.RefreshToken {
		t.Fatal("refresh token should rotate")
	}
	if _, err = jwt.ParseWithClaims(rotated.AccessToken, jwt.MapClaims{}, func(tok *jwt.Token) (any, error) { return pub, nil }); err != nil {
		t.Fatalf("rotated access token invalid: %v", err)
	}
	// 旧 refresh 重放 → 家族吊销 → 新 refresh 也失效
	if _, err = mgr.Refresh(context.Background(), pair.RefreshToken, "fp-123"); err == nil {
		t.Fatal("replayed old refresh should be rejected")
	}
	if _, err = mgr.Refresh(context.Background(), rotated.RefreshToken, "fp-123"); err == nil {
		t.Fatal("family revocation should invalidate the rotated token too")
	}
}

func TestSessionManager_RefreshFingerprintMismatchRevokesFamily(t *testing.T) {
	mgr, accounts, _ := newTestSessionManager(t)
	account, _ := accounts.FindByID(context.Background(), 1, 7)
	pair, err := mgr.CreateSessionForAccount(context.Background(), account, "fp-123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = mgr.Refresh(context.Background(), pair.RefreshToken, "fp-OTHER"); err == nil {
		t.Fatal("fingerprint mismatch should be rejected")
	}
	if _, err = mgr.Refresh(context.Background(), pair.RefreshToken, "fp-123"); err == nil {
		t.Fatal("fingerprint mismatch should revoke the family")
	}
}

func TestSessionManager_LogoutAndRevokeAccountSessions(t *testing.T) {
	mgr, accounts, _ := newTestSessionManager(t)
	account, _ := accounts.FindByID(context.Background(), 1, 7)
	pair, err := mgr.CreateSessionForAccount(context.Background(), account, "fp-123")
	if err != nil {
		t.Fatal(err)
	}
	if err = mgr.Logout(context.Background(), pair.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err = mgr.Refresh(context.Background(), pair.RefreshToken, "fp-123"); err == nil {
		t.Fatal("logged-out session should not refresh")
	}

	pair2, err := mgr.CreateSessionForAccount(context.Background(), account, "fp-123")
	if err != nil {
		t.Fatal(err)
	}
	n, err := mgr.RevokeAccountSessions(context.Background(), 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 revoked session, got %d", n)
	}
	if _, err = mgr.Refresh(context.Background(), pair2.RefreshToken, "fp-123"); err == nil {
		t.Fatal("revoked session should not refresh")
	}
}

func TestSessionManager_RejectsDisabledAccount(t *testing.T) {
	mgr, accounts, _ := newTestSessionManager(t)
	account, _ := accounts.FindByID(context.Background(), 1, 7)
	account.Disabled = true
	if _, err := mgr.CreateSessionForAccount(context.Background(), account, "fp-123"); err != ErrInvalidCredentials {
		t.Fatalf("disabled account should be rejected, got %v", err)
	}
}

func TestSessionManager_RejectsInvalidConfig(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := NewSessionManager(nil, newMemSessions(), DefaultArgon2idHasher(),
		priv, "i", "a", time.Minute, time.Minute); err == nil {
		t.Fatal("nil account store should be rejected")
	}
	if _, err := NewSessionManager(newMemAccounts(), newMemSessions(), DefaultArgon2idHasher(),
		priv, "i", "a", time.Hour, time.Minute); err == nil {
		t.Fatal("refreshTTL < accessTTL should be rejected")
	}
}
