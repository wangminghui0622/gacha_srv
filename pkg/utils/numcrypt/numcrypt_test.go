package numcrypt

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	c, err := New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	const n = uint64(1234567890123456)
	got, err := c.Encrypt(n)
	if err != nil {
		t.Fatal(err)
	}
	if got != "EnR5ZBuCp" {
		t.Fatalf("Encrypt() = %q, want EnR5ZBuCp", got)
	}
	back, err := c.Decrypt(got)
	if err != nil {
		t.Fatal(err)
	}
	if back != n {
		t.Fatalf("Decrypt() = %d, want %d", back, n)
	}
}
