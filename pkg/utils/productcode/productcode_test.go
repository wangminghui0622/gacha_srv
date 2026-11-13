package productcode

import (
	"strings"
	"testing"
)

func TestGenerateDecode(t *testing.T) {
	const productID = uint64(800000001)
	code, err := Generate(productID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, "P") {
		t.Fatalf("Generate() = %q, want P prefix", code)
	}
	back, err := Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if back != productID {
		t.Fatalf("Decode() = %d, want %d", back, productID)
	}
}
