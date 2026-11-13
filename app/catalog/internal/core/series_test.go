package core

import (
	"strings"
	"testing"
)

func TestBuildSeries(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		brandID   uint64
		weight    int32
		wantName  string
		wantError bool
	}{
		{name: "trim", input: "  KOJUN WORKS  ", brandID: 2, weight: 10, wantName: "KOJUN WORKS"},
		{name: "name limit", input: strings.Repeat("系", SeriesNameMaxLength), brandID: 1, wantName: strings.Repeat("系", SeriesNameMaxLength)},
		{name: "empty name", input: "  ", brandID: 1, wantError: true},
		{name: "name too long", input: strings.Repeat("系", SeriesNameMaxLength+1), brandID: 1, wantError: true},
		{name: "name control char", input: "KO\nJUN", brandID: 1, wantError: true},
		{name: "brand required", input: "KOJUN WORKS", brandID: 0, weight: 10, wantError: true},
		{name: "negative weight", input: "KOJUN WORKS", brandID: 1, weight: -1, wantError: true},
		{name: "weight too large", input: "KOJUN WORKS", brandID: 1, weight: SeriesSortWeightLimit + 1, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, _, _, err := BuildSeries(tt.input, tt.brandID, tt.weight)
			if tt.wantError {
				if err == nil {
					t.Fatalf("BuildSeries() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildSeries() error = %v", err)
			}
			if gotName != tt.wantName {
				t.Fatalf("BuildSeries() = %q, want %q", gotName, tt.wantName)
			}
		})
	}
}
