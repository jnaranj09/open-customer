package text

import (
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
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

func TestNotes(t *testing.T) {
	s := func(v string) *string { return &v }
	if got, ok := Notes(nil); got != nil || !ok {
		t.Errorf("nil: got %v, %v; want nil, true", got, ok)
	}
	if got, ok := Notes(s("   ")); got != nil || !ok {
		t.Errorf("blank: got %q, %v; want nil, true", *got, ok)
	}
	if got, ok := Notes(s("  Prefiere tardes ")); got == nil || *got != "Prefiere tardes" || !ok {
		t.Errorf("text: got %v, %v; want trimmed, true", got, ok)
	}
	if got, ok := Notes(s(strings.Repeat("ñ", MaxNotes))); got == nil || !ok {
		t.Errorf("MaxNotes runes: got %v, %v; want accepted", got, ok)
	}
	if got, ok := Notes(s(strings.Repeat("a", MaxNotes+1))); got != nil || ok {
		t.Errorf("MaxNotes+1 runes: got %v, %v; want nil, false", got, ok)
	}
}

func TestLine(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string // "" means nil
		wantMsg string
	}{
		{"surrounding spaces trimmed", "  Incluye retiro ", "Incluye retiro", ""},
		{"blank is nil", "   ", "", ""},
		{"200 runes of two bytes", strings.Repeat("ñ", 200), strings.Repeat("ñ", 200), ""},
		{"201 runes", strings.Repeat("a", 201), "", problem.MsgTextTooLong},
		{"a line break", "Incluye\nretiro", "", problem.MsgTextOneLine},
		{"a carriage return", "Incluye\rretiro", "", problem.MsgTextOneLine},
		{"a trailing line break is trimmed", "Incluye retiro\n", "Incluye retiro", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, msg := Line(tt.in, 200)
			gotS := ""
			if got != nil {
				gotS = *got
			}
			if gotS != tt.want || msg != tt.wantMsg {
				t.Errorf("Line(%q) = %q, %q; want %q, %q", tt.in, gotS, msg, tt.want, tt.wantMsg)
			}
		})
	}
}
