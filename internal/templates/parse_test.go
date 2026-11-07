package templates

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

func bodyErrors(t *testing.T, err error) []problem.FieldError {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("error = %v, want problem.Validation", err)
	}
	return v.Errors
}

func unclosed(pos int) problem.FieldError {
	return problem.FieldError{Field: "body", Message: fmt.Sprintf(problem.MsgTemplateUnclosed, pos)}
}

func ptr(s string) *string { return &s }

func TestParseVars(t *testing.T) {
	got, err := Parse(`Hola {{client.first_name}}, te esperamos con {{ client.bebida_favorita | "un café" }}`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []VarRef{
		{NS: "client", Key: "first_name", Pos: 6},
		{NS: "client", Key: "bebida_favorita", Fallback: ptr("un café"), Pos: 46},
	}
	if !reflect.DeepEqual(got.Vars, want) {
		t.Errorf("Vars = %+v, want %+v", got.Vars, want)
	}
}

func TestParseAcceptsSpacesAndEmptyFallback(t *testing.T) {
	got, err := Parse(`{{  business.name  }}{{client.x|""}}{{client.y|  "a b"  }}`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []VarRef{
		{NS: "business", Key: "name", Pos: 1},
		{NS: "client", Key: "x", Fallback: ptr(""), Pos: 22},
		{NS: "client", Key: "y", Fallback: ptr("a b"), Pos: 37},
	}
	if !reflect.DeepEqual(got.Vars, want) {
		t.Errorf("Vars = %+v, want %+v", got.Vars, want)
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unclosed", "{{client.name"},
		{"no namespace", "{{ hola }}"},
		{"empty key", "{{client.}}"},
		{"curly quotes", "{{client.name | “x” }}"},
		{"curly open straight close", "{{client.name | “x\" }}"},
		{"straight open curly close", "{{client.name | \"x” }}"},
		{"unclosed quote", "{{client.name | \"x }}"},
		{"tab is not a space", "{{\tclient.name}}"},
		{"uppercase", "{{Client.name}}"},
		{"key starts with digit", "{{client.1a}}"},
		{"fallback without bar", `{{client.name "x"}}`},
		{"junk after fallback", `{{client.name | "x" y}}`},
		{"empty braces", "{{}}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.body)
			got := bodyErrors(t, err)
			want := []problem.FieldError{unclosed(1)}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("errors = %+v, want %+v", got, want)
			}
		})
	}
}

func TestParseCollectsEveryError(t *testing.T) {
	_, err := Parse("{{ a }} ok {{client.name}} {{client.")
	got := bodyErrors(t, err)
	want := []problem.FieldError{unclosed(1), unclosed(28)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("errors = %+v, want %+v", got, want)
	}
}

func TestParseReturnsPartialVarsWithError(t *testing.T) {
	got, err := Parse("{{client.name}} {{")
	if errs := bodyErrors(t, err); len(errs) != 1 {
		t.Errorf("errors = %+v, want one", errs)
	}
	want := []VarRef{{NS: "client", Key: "name", Pos: 1}}
	if !reflect.DeepEqual(got.Vars, want) {
		t.Errorf("Vars = %+v, want %+v", got.Vars, want)
	}
}

func TestParseUnclosedPositionCountsRunes(t *testing.T) {
	_, err := Parse("Hola, Lucía {{")
	got := bodyErrors(t, err)
	if want := []problem.FieldError{unclosed(13)}; !reflect.DeepEqual(got, want) {
		t.Errorf("errors = %+v, want %+v", got, want)
	}
}

func TestParseScansAfterTheBadBraces(t *testing.T) {
	// The scan resumes two characters after the bad "{{", so a valid token right behind it is found.
	got, err := Parse("{{{client.name}}")
	if errs := bodyErrors(t, err); !reflect.DeepEqual(errs, []problem.FieldError{unclosed(1)}) {
		t.Errorf("errors = %+v", errs)
	}
	if len(got.Vars) != 0 {
		t.Errorf("Vars = %+v, want none", got.Vars)
	}
}

func TestParsePlainTextIsFine(t *testing.T) {
	got, err := Parse("}} { } 💅 sin variables")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.Vars) != 0 {
		t.Errorf("Vars = %+v, want none", got.Vars)
	}
}

func TestParseManyBadBracesIsLinearAndOrdered(t *testing.T) {
	const n = 1000
	_, err := Parse(strings.Repeat("{{", n))
	got := bodyErrors(t, err)
	if len(got) != 20 {
		t.Fatalf("got %d errors, want the first 20", len(got))
	}
	for i, e := range got {
		if want := unclosed(2*i + 1); e != want {
			t.Fatalf("error %d = %+v, want %+v", i, e, want)
		}
	}
}

func TestParseKeepsVarsPastTheErrorCap(t *testing.T) {
	got, err := Parse(strings.Repeat("{{ ", 30) + "{{client.name}}")
	if errs := bodyErrors(t, err); len(errs) != 20 {
		t.Errorf("got %d errors, want 20", len(errs))
	}
	want := []VarRef{{NS: "client", Key: "name", Pos: 91}}
	if !reflect.DeepEqual(got.Vars, want) {
		t.Errorf("Vars = %+v, want %+v", got.Vars, want)
	}
}
