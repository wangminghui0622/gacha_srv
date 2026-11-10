package utils

import (
	"errors"
	"testing"
)

func TestYuanToFen(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"0", 0, true},
		{"1", 100, true},
		{"10", 1000, true},
		{"10.5", 1050, true},
		{"10.50", 1050, true},
		{"0.01", 1, true},
		{"99999999.99", 9999999999, true},
		{"", 0, false},
		{"-1", 0, false},
		{"1.234", 0, false},
		{"1.", 0, false},
		{".5", 0, false},
		{"abc", 0, false},
		{"1,000", 0, false},
	}
	for _, c := range cases {
		got, err := YuanToFen(c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Errorf("YuanToFen(%q) = (%d, %v), want (%d, nil)", c.in, got, err, c.want)
			}
		} else {
			if err == nil || !errors.Is(err, ErrMoneyFormat) {
				t.Errorf("YuanToFen(%q) = (%d, %v), want ErrMoneyFormat", c.in, got, err)
			}
		}
	}
}

func TestFenToYuan(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{100, "1.00"},
		{1050, "10.50"},
		{9999999999, "99999999.99"},
		{-1, "-0.01"},
		{-1050, "-10.50"},
	}
	for _, c := range cases {
		if got := FenToYuan(c.in); got != c.want {
			t.Errorf("FenToYuan(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 往返：合法输入 YuanToFen → FenToYuan 不丢精度
func TestRoundTrip(t *testing.T) {
	for _, in := range []string{"0.01", "12.34", "1000.00", "0.99", "888888.88"} {
		fen, err := YuanToFen(in)
		if err != nil {
			t.Fatalf("YuanToFen(%q) unexpected error: %v", in, err)
		}
		if got := FenToYuan(fen); got != in {
			t.Errorf("round trip %q -> %q", in, got)
		}
	}
}
