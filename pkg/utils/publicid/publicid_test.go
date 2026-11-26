package publicid

import "testing"

func TestUserIDRoundTrip(t *testing.T) {
	for _, id := range []int64{1, 1000001, 1000002, 1234567890123456} {
		encoded, err := EncodeUserID(id)
		if err != nil {
			t.Fatalf("EncodeUserID(%d): %v", id, err)
		}
		if encoded[0] != 'u' {
			t.Fatalf("EncodeUserID(%d) = %q, want u prefix", id, encoded)
		}
		decoded, err := DecodeUserID(encoded)
		if err != nil {
			t.Fatalf("DecodeUserID(%q): %v", encoded, err)
		}
		if decoded != id {
			t.Fatalf("DecodeUserID(EncodeUserID(%d)) = %d", id, decoded)
		}
	}
}

func TestParseUserID(t *testing.T) {
	encoded, err := EncodeUserID(1000001)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"1000001", encoded} {
		id, err := ParseUserID(input)
		if err != nil {
			t.Fatalf("ParseUserID(%q): %v", input, err)
		}
		if id != 1000001 {
			t.Fatalf("ParseUserID(%q) = %d", input, id)
		}
	}
	if _, err := ParseUserID("not-a-user"); err == nil {
		t.Fatal("ParseUserID should reject invalid input")
	}
}

func TestDecodeRejectsMissingPrefix(t *testing.T) {
	if _, err := DecodeUserID("abc123"); err == nil {
		t.Fatal("DecodeUserID should require u prefix")
	}
}
