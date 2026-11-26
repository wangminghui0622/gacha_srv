package ordercode

import "testing"

func TestOrderCodeRoundTrip(t *testing.T) {
	for _, seq := range []uint64{333000000, 333000001, 333000002, 333000003} {
		code, err := Encode(seq)
		if err != nil {
			t.Fatalf("Encode(%d): %v", seq, err)
		}
		if code[0] != 'o' {
			t.Fatalf("Encode(%d) = %q, want o prefix", seq, code)
		}
		decoded, err := Decode(code)
		if err != nil {
			t.Fatalf("Decode(%q): %v", code, err)
		}
		if decoded != seq {
			t.Fatalf("Decode(Encode(%d)) = %d", seq, decoded)
		}
	}
}

func TestOrderCodeRejectsInvalid(t *testing.T) {
	if _, err := Encode(0); err == nil {
		t.Fatal("Encode(0) should fail")
	}
	if _, err := Decode("abc123"); err == nil {
		t.Fatal("Decode should require o prefix")
	}
}
