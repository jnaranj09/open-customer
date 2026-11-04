package text

import (
	"strings"
	"testing"
)

func TestTrim(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		wantOK bool
		min    int
		max    int
	}{
		{"surrounding spaces", "  Lucía ", "Lucía", true, 1, 80},
		{"only spaces", "   ", "", false, 1, 80},
		{"81 runes", strings.Repeat("a", 81), strings.Repeat("a", 81), false, 1, 80},
		{"80 runes of two bytes", strings.Repeat("ñ", 80), strings.Repeat("ñ", 80), true, 1, 80},
		{"empty allowed when min is 0", "  ", "", true, 0, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Trim(tt.in, tt.min, tt.max)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("Trim(%q, %d, %d) = %q, %v; want %q, %v", tt.in, tt.min, tt.max, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
