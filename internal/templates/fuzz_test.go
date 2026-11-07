package templates

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// FuzzParseTemplate checks the template scanner on any body: Parse and Render never panic; Parse
// either returns nil or a problem.Validation whose errors are all on field "body"; every variable's
// position is inside the body; and a body that parses renders, with a value for every variable, to
// text that parses again with no variable and no error (the render holds no token).
func FuzzParseTemplate(f *testing.F) {
	for _, seed := range []string{
		"{{client.name", "{{ hola }}", "{{client.}}", "{{client.name | “x” }}", "{{client.name | “x\" }}",
		"{{client.name | \"x” }}", "{{client.name | \"x }}", "{{\tclient.name}}", "{{Client.name}}",
		"{{client.1a}}", `{{client.name "x"}}`, `{{client.name | "x" y}}`, "{{}}",
		"Hola {{client.first_name}}, tu cita es {{appointment.date | \"pronto\"}}.", "{{{client.name}}",
		"{{client.name}}}}", "{{", "{", "\x00{{client.name}}", "\xff{{a.b}}", "{{a.b | \"{{\"}}",
		"{{ client.name | \"\" }}", strings.Repeat("{{", 500),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		parsed, err := Parse(body)
		_ = Render(body, Data{})
		if err != nil {
			var v problem.Validation
			if !errors.As(err, &v) || len(v.Errors) == 0 {
				t.Fatalf("Parse error = %v, want a problem.Validation with errors", err)
			}
			for _, fe := range v.Errors {
				if fe.Field != "body" {
					t.Fatalf("error field = %q, want body", fe.Field)
				}
			}
		}
		runes := utf8.RuneCountInString(body)
		data := Data{Client: map[string]string{}, Appointment: map[string]string{}, Business: map[string]string{}}
		known := true
		for _, v := range parsed.Vars {
			if v.Pos < 1 || v.Pos > runes {
				t.Fatalf("variable position %d is outside the body of %d runes", v.Pos, runes)
			}
			switch v.NS {
			case nsClient:
				data.Client[v.Key] = "V"
			case nsAppointment:
				data.Appointment[v.Key] = "V"
			case nsBusiness:
				data.Business[v.Key] = "V"
			default:
				// An unknown namespace renders its fallback, which may hold "{{".
				known = false
			}
		}
		if err != nil || !known {
			return
		}
		again, err := Parse(Render(body, data))
		if err != nil || len(again.Vars) != 0 {
			t.Fatalf("render of a parsed body parses again with %d variables and error %v", len(again.Vars), err)
		}
	})
}
