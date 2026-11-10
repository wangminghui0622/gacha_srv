package phonecrypto

import (
	"bytes"
	"strings"
	"testing"
)

func testCodec(t *testing.T) *Codec {
	t.Helper()
	c, err := New(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32), "enc-v1", 1)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSealAndBlindIndex(t *testing.T) {
	c := testCodec(t)
	a, err := c.Seal(10, 20, "+8613800123456")
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.Seal(10, 20, "13800123456")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a.Ciphertext, b.Ciphertext) || a.ExactToken != b.ExactToken || len(a.GramTokens) != 8 {
		t.Fatal("randomized ciphertext and stable blind index required")
	}
	phone, err := c.Open(10, 20, a)
	if err != nil || phone != "13800123456" {
		t.Fatalf("decrypted %q: %v", phone, err)
	}
	tokens, err := c.SearchTokens(10, "0012")
	if err != nil || len(tokens) != 1 || tokens[0] != a.GramTokens[3] {
		t.Fatalf("fragment token mismatch: %v", err)
	}
	if _, err = c.Open(11, 20, a); err == nil {
		t.Fatal("cross-tenant decryption succeeded")
	}
	if _, err = c.Open(10, 21, a); err == nil {
		t.Fatal("cross-user decryption succeeded")
	}
	a.Ciphertext[0] ^= 1
	if _, err = c.Open(10, 20, a); err == nil {
		t.Fatal("tampered ciphertext decrypted")
	}
}

func TestSearchInputAndTenantIsolation(t *testing.T) {
	c := testCodec(t)
	for _, input := range []string{"138", "13x0", strings.Repeat("1", 12)} {
		if _, err := c.SearchTokens(1, input); err == nil {
			t.Fatalf("accepted invalid fragment %q", input)
		}
	}
	a, _ := c.SearchTokens(1, "1234")
	b, _ := c.SearchTokens(2, "1234")
	if a[0] == b[0] {
		t.Fatal("tenant token collision")
	}
	parts, _ := c.SearchTokens(1, "11111")
	if len(parts) != 1 {
		t.Fatalf("duplicate grams not deduplicated: %d", len(parts))
	}
}

func TestRejectSameKeyAndInvalidPhone(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	if _, err := New(key, key, "enc-v1", 1); err == nil {
		t.Fatal("same key accepted")
	}
	if _, err := testCodec(t).Seal(1, 2, "123"); err == nil {
		t.Fatal("invalid phone accepted")
	}
}
