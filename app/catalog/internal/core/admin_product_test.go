package core

import (
	"strings"
	"testing"
)

func TestBuildAdminProduct(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		code      string
		saleType  string
		cover     string
		cost      int64
		sale      int64
		wantType  string
		wantError bool
	}{
		{name: "defaults", title: " Hirono 手办 ", code: "PM-001", saleType: "", cover: "", cost: 100, sale: 200, wantType: SaleTypeInStock},
		{name: "preorder", title: "Hirono 手办", saleType: SaleTypePreOrder, wantType: SaleTypePreOrder},
		{name: "empty title", title: " ", wantError: true},
		{name: "title too long", title: strings.Repeat("品", AdminProductTitleMaxLength+1), wantError: true},
		{name: "code too long", title: "商品", code: strings.Repeat("A", AdminProductCodeMaxLength+1), wantError: true},
		{name: "bad sale type", title: "商品", saleType: "UNKNOWN", wantError: true},
		{name: "negative cost", title: "商品", cost: -1, wantError: true},
		{name: "negative sale", title: "商品", sale: -1, wantError: true},
		{name: "cover too long", title: "商品", cover: strings.Repeat("x", AdminProductCoverMaxLength+1), wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, saleType, _, err := BuildAdminProduct(tt.title, tt.code, tt.saleType, tt.cover, tt.cost, tt.sale)
			if tt.wantError {
				if err == nil {
					t.Fatalf("BuildAdminProduct() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildAdminProduct() error = %v", err)
			}
			if saleType != tt.wantType {
				t.Fatalf("BuildAdminProduct() saleType = %q, want %q", saleType, tt.wantType)
			}
		})
	}
}
