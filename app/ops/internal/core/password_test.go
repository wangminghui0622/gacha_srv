package core

import (
	"strings"
	"testing"
)

func TestArgon2idHasher_Roundtrip(t *testing.T) {
	h := DefaultArgon2idHasher()
	hash, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}
	if !h.Verify(hash, "correct horse battery staple") {
		t.Fatal("Verify should accept the original password")
	}
	if h.Verify(hash, "wrong password here") {
		t.Fatal("Verify should reject a wrong password")
	}
}

func TestArgon2idHasher_RejectsShortPassword(t *testing.T) {
	if _, err := DefaultArgon2idHasher().Hash("short"); err == nil {
		t.Fatal("Hash should reject passwords under 12 bytes")
	}
}

func TestArgon2idHasher_RejectsMalformed(t *testing.T) {
	h := DefaultArgon2idHasher()
	for _, bad := range []string{"", "plain", "$argon2id$v=19$m=1,t=1,p=1$salt$hash", "$argon2id$v=18$m=1,t=1,p=1$c2FsdA$aGFzaA"} {
		if h.Verify(bad, "whatever password") {
			t.Fatalf("Verify should reject malformed hash %q", bad)
		}
	}
}
