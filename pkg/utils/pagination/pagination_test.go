package pagination

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name         string
		page         uint32
		pageSize     uint32
		fallback     uint32
		wantPage     uint32
		wantPageSize uint32
	}{
		{name: "defaults", page: 0, pageSize: 0, fallback: 15, wantPage: 1, wantPageSize: 15},
		{name: "keep explicit", page: 3, pageSize: 30, fallback: 15, wantPage: 3, wantPageSize: 30},
		{name: "clamp max", page: 2, pageSize: 500, fallback: 15, wantPage: 2, wantPageSize: MaxPageSize},
		{name: "fallback 20", page: 0, pageSize: 0, fallback: 20, wantPage: 1, wantPageSize: 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotSize := Normalize(tt.page, tt.pageSize, tt.fallback)
			if gotPage != tt.wantPage || gotSize != tt.wantPageSize {
				t.Fatalf("Normalize() = (%d, %d), want (%d, %d)", gotPage, gotSize, tt.wantPage, tt.wantPageSize)
			}
		})
	}
}

func TestNormalizeSize(t *testing.T) {
	if got := NormalizeSize(0, 20); got != 20 {
		t.Fatalf("NormalizeSize() = %d, want 20", got)
	}
	if got := NormalizeSize(1000, 20); got != MaxPageSize {
		t.Fatalf("NormalizeSize() = %d, want %d", got, MaxPageSize)
	}
	if got := NormalizeSize(0, 0); got != 20 {
		t.Fatalf("NormalizeSize() with empty fallback = %d, want 20", got)
	}
}
