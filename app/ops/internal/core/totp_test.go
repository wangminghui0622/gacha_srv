package core

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 附录 B 测试向量（SHA1）：密钥 ASCII "12345678901234567890"（base32
// GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ），t=59s → 8 位 94287082 → 6 位 287082。
const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestGenerateTotpCode_RFC6238Vector(t *testing.T) {
	got := GenerateTotpCode(rfcSecret, time.Unix(59, 0))
	if got != "287082" {
		t.Fatalf("RFC 6238 vector failed: got %s want 287082", got)
	}
}

func TestValidateTotpCode_Window(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	secret, err := GenerateTotpSecret()
	if err != nil {
		t.Fatalf("GenerateTotpSecret: %v", err)
	}
	current := GenerateTotpCode(secret, now)
	prev := GenerateTotpCode(secret, now.Add(-30*time.Second))
	next := GenerateTotpCode(secret, now.Add(30*time.Second))
	for _, code := range []string{current, prev, next} {
		if !ValidateTotpCode(secret, code, now) {
			t.Fatalf("ValidateTotpCode should accept window code %s", code)
		}
	}
	if ValidateTotpCode(secret, "000000", now) && current != "000000" {
		t.Fatal("ValidateTotpCode should reject an unrelated code")
	}
	if ValidateTotpCode(secret, "12345", now) || ValidateTotpCode(secret, "", now) {
		t.Fatal("ValidateTotpCode should reject non-6-digit codes")
	}
}

func TestGenerateTotpSecret_Base32NoPadding(t *testing.T) {
	secret, err := GenerateTotpSecret()
	if err != nil {
		t.Fatalf("GenerateTotpSecret: %v", err)
	}
	if secret == "" || strings.Contains(secret, "=") {
		t.Fatalf("secret must be non-empty base32 without padding, got %q", secret)
	}
}

func TestBuildTotpURI(t *testing.T) {
	uri := BuildTotpURI("Gacha Console", "admin", rfcSecret)
	if !strings.HasPrefix(uri, "otpauth://totp/Gacha%20Console:admin?") {
		t.Fatalf("unexpected URI: %s", uri)
	}
	for _, fragment := range []string{"secret=" + rfcSecret, "issuer=Gacha+Console", "algorithm=SHA1", "digits=6", "period=30"} {
		if !strings.Contains(uri, fragment) {
			t.Fatalf("URI missing %s: %s", fragment, uri)
		}
	}
}
