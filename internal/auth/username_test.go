package auth

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

func TestNormalizeUsername(t *testing.T) {
	t.Parallel()
	accepted := []struct {
		name string
		in   string
		want string
	}{
		{"surrounding spaces are trimmed", " Ana ", "ana"},
		{"capitals are lowercased and . _ - are kept", "ANA.b_c-d", "ana.b_c-d"},
		{"3 characters are the minimum", "abc", "abc"},
		{"32 characters are the maximum", strings.Repeat("a", 32), strings.Repeat("a", 32)},
		{"digits are allowed", "ana2026", "ana2026"},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeUsername(tt.in)
			if err != nil || got != tt.want {
				t.Fatalf("normalizeUsername(%q) = %q, %v; want %q, nil", tt.in, got, err, tt.want)
			}
		})
	}

	want := problem.Validation{Errors: []problem.FieldError{
		{Field: "username", Message: problem.MsgUsernameInvalid},
	}}
	rejected := []struct {
		name string
		in   string
	}{
		{"a space inside is rejected", "an a"},
		{"2 characters are rejected", "ab"},
		{"2 characters after trimming are rejected", "  ab  "},
		{"33 characters are rejected", strings.Repeat("a", 33)},
		{"a letter outside a-z is rejected", "año"},
		{"other punctuation is rejected", "ana@b"},
		{"an empty name is rejected", ""},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeUsername(tt.in)
			var v problem.Validation
			if !errors.As(err, &v) || !reflect.DeepEqual(v, want) || got != "" {
				t.Fatalf("normalizeUsername(%q) = %q, %v; want \"\" and %+v", tt.in, got, err, want)
			}
		})
	}
}
