package color

import "testing"

func TestNormalizeColor(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{"uppercase is lowercased", "#AA3366", "#aa3366", true},
		{"short form", "#abc", "", false},
		{"no hash", "aa3366", "", false},
		{"not hex", "#gg0000", "", false},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Normalize(tt.in)
			if ok != tt.wantOK || (ok && got != tt.want) {
				t.Errorf("Normalize(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
