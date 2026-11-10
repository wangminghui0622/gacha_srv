package jwtutil

import "testing"

const (
	testSecret = "test-secret"
	expire     = 7200
)

func TestTokenRoundTrip(t *testing.T) {
	token, err := NewToken(testSecret, expire, 42, "alice", "爱丽丝")
	if err != nil {
		t.Fatalf("NewToken failed: %v", err)
	}
	claims, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.UserId != 42 || claims.Username != "alice" || claims.Nickname != "爱丽丝" {
		t.Errorf("claims mismatch: %+v", claims)
	}
	if claims.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil")
	}
}

func TestParseTokenWrongSecret(t *testing.T) {
	token, err := NewToken(testSecret, expire, 1, "a", "a")
	if err != nil {
		t.Fatalf("NewToken failed: %v", err)
	}
	if _, err := ParseToken("other-secret", token); err == nil {
		t.Error("ParseToken with wrong secret should fail")
	}
}

func TestParseTokenGarbage(t *testing.T) {
	if _, err := ParseToken(testSecret, "not-a-token"); err == nil {
		t.Error("ParseToken with garbage should fail")
	}
	if _, err := ParseToken(testSecret, ""); err == nil {
		t.Error("ParseToken with empty token should fail")
	}
}
