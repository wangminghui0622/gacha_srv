package core

import (
	"regexp"
	"testing"
)

func TestNormalizeCNMobile(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "13800001111", want: "13800001111"},
		{in: " 13800001111 ", want: "13800001111"},
		{in: "+8613800001111", want: "13800001111"},
		{in: "+86 13800001111", want: "13800001111"},
		{in: "19912345678", want: "19912345678"},
		{in: "", wantErr: true},
		{in: "12800001111", wantErr: true},  // 1[2] 段非法
		{in: "1380000111", wantErr: true},   // 10 位
		{in: "138000011112", wantErr: true}, // 12 位
		{in: "8613800001111", wantErr: true},
		{in: "abc13800001111", wantErr: true},
	}
	for _, c := range cases {
		got, err := NormalizeCNMobile(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalizeCNMobile(%q) expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("NormalizeCNMobile(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestRandomDigits(t *testing.T) {
	digits := regexp.MustCompile(`^[0-9]+$`)
	for _, size := range []int{6, 4} {
		s, err := RandomDigits(size)
		if err != nil {
			t.Fatalf("RandomDigits(%d) error: %v", size, err)
		}
		if len(s) != size || !digits.MatchString(s) {
			t.Errorf("RandomDigits(%d) = %q; want %d-digit numeric", size, s, size)
		}
	}
	// 不同调用不应恒定相同（随机性冒烟）
	a, _ := RandomDigits(6)
	b, _ := RandomDigits(6)
	if a == b {
		t.Log("RandomDigits returned identical strings (unlikely but possible)")
	}
}
