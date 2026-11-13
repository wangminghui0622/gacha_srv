package core

import (
	"strings"
	"testing"
)

func TestBuildBrand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		logo      string
		wantName  string
		wantLogo  string
		wantError bool
	}{
		{name: "trim", input: "  盖世小鸡  ", logo: "  https://cdn.example/logo.png  ", wantName: "盖世小鸡", wantLogo: "https://cdn.example/logo.png"},
		{name: "empty logo", input: "vivo", logo: "", wantName: "vivo", wantLogo: ""},
		{name: "name limit", input: strings.Repeat("牌", BrandNameMaxLength), wantName: strings.Repeat("牌", BrandNameMaxLength)},
		{name: "empty name", input: "  ", wantError: true},
		{name: "name too long", input: strings.Repeat("牌", BrandNameMaxLength+1), wantError: true},
		{name: "logo too long", input: "vivo", logo: strings.Repeat("x", BrandLogoMaxLength+1), wantError: true},
		{name: "name control char", input: "vi\nvo", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotLogo, err := BuildBrand(tt.input, tt.logo)
			if tt.wantError {
				if err == nil {
					t.Fatalf("BuildBrand() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildBrand() error = %v", err)
			}
			if gotName != tt.wantName || gotLogo != tt.wantLogo {
				t.Fatalf("BuildBrand() = (%q, %q), want (%q, %q)", gotName, gotLogo, tt.wantName, tt.wantLogo)
			}
		})
	}
}
