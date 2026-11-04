package attributes

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var keyShape = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func TestGenerateKey(t *testing.T) {
	tests := []struct {
		name  string
		label string
		taken map[string]bool
		want  string
	}{
		{"plain", "Bebida favorita", nil, "bebida_favorita"},
		{"accents and double space", "Ñandú  Azul", nil, "nandu_azul"},
		{"punctuation", "¿Alergias?", nil, "alergias"},
		{"leading digit", "2do color", nil, "a_2do_color"},
		{"nothing usable", "¡!", nil, "a"},
		{"reserved", "Name", nil, "name_2"},
		{"taken", "Bebida favorita", map[string]bool{"bebida_favorita": true}, "bebida_favorita_2"},
		{"taken twice", "Bebida favorita", map[string]bool{"bebida_favorita": true, "bebida_favorita_2": true}, "bebida_favorita_3"},
		{"cut to 40", strings.Repeat("a", 60), nil, strings.Repeat("a", 40)},
		{"cut does not end in underscore", strings.Repeat("a", 39) + " bb", nil, strings.Repeat("a", 39)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateKey(tt.label, tt.taken)
			if got != tt.want {
				t.Errorf("GenerateKey(%q) = %q, want %q", tt.label, got, tt.want)
			}
			if !keyShape.MatchString(got) {
				t.Errorf("key %q does not match %s", got, keyShape)
			}
		})
	}
}

func TestGenerateKeyReservedWords(t *testing.T) {
	for _, word := range []string{"name", "first_name", "phone", "notes", "date", "time", "services", "id"} {
		t.Run(word, func(t *testing.T) {
			if got, want := GenerateKey(word, nil), word+"_2"; got != want {
				t.Errorf("GenerateKey(%q) = %q, want %q", word, got, want)
			}
		})
	}
}

func TestParseScopeAndType(t *testing.T) {
	for _, s := range []string{"client", "appointment"} {
		if got, err := ParseScope(s); err != nil || string(got) != s {
			t.Errorf("ParseScope(%q) = %q, %v", s, got, err)
		}
	}
	for _, s := range []string{"text", "number", "date", "select", "multiselect"} {
		if got, err := ParseType(s); err != nil || string(got) != s {
			t.Errorf("ParseType(%q) = %q, %v", s, got, err)
		}
	}
	assertField(t, parseScopeErr("service"), "scope", problem.MsgScopeInvalid)
	for _, s := range []string{"image", "payment_method", ""} {
		_, err := ParseType(s)
		assertField(t, err, "type", problem.MsgAttrTypeInvalid)
	}
}

func parseScopeErr(s string) error {
	_, err := ParseScope(s)
	return err
}

func assertField(t *testing.T, err error, field, message string) {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) || len(v.Errors) != 1 || v.Errors[0].Field != field || v.Errors[0].Message != message {
		t.Errorf("error = %#v, want one validation error on %q with %q", err, field, message)
	}
}
