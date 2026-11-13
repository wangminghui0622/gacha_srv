package core

import (
	"strings"
	"testing"
)

func TestBuildIP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		weight   int32
		wantName string
		wantErr  bool
	}{
		{name: "trim", input: "  布兰兔  ", weight: 44, wantName: "布兰兔"},
		{name: "unicode limit", input: strings.Repeat("兔", IPNameMaxLength), weight: 0, wantName: strings.Repeat("兔", IPNameMaxLength)},
		{name: "empty", input: "  ", weight: 0, wantErr: true},
		{name: "too long", input: strings.Repeat("兔", IPNameMaxLength+1), weight: 0, wantErr: true},
		{name: "negative weight", input: "布兰兔", weight: -1, wantErr: true},
		{name: "weight too large", input: "布兰兔", weight: IPSortWeightLimit + 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := BuildIP(tt.input, tt.weight)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("BuildIP() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildIP() error = %v", err)
			}
			if got != tt.wantName {
				t.Fatalf("BuildIP() = %q, want %q", got, tt.wantName)
			}
		})
	}
}
