package attributes

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var (
	optA   = uuid.MustParse("00000000-0000-7000-8000-00000000000a")
	optB   = uuid.MustParse("00000000-0000-7000-8000-00000000000b")
	optOld = uuid.MustParse("00000000-0000-7000-8000-0000000000ff")
	optOth = uuid.MustParse("00000000-0000-7000-8000-0000000000aa")
)

func opts() []Option {
	return []Option{
		{ID: optA, Label: "A"},
		{ID: optB, Label: "B"},
		{ID: optOld, Label: "Vieja", Archived: true},
	}
}

func defs() []Attribute {
	return []Attribute{
		{Key: "nota", Label: "Nota", Scope: ScopeClient, Type: TypeText},
		{Key: "peso", Label: "Peso", Scope: ScopeClient, Type: TypeNumber},
		{Key: "cumple", Label: "Cumpleaños", Scope: ScopeClient, Type: TypeDate},
		{Key: "color", Label: "Color", Scope: ScopeClient, Type: TypeSelect, Options: opts()},
		{Key: "gustos", Label: "Gustos", Scope: ScopeClient, Type: TypeMultiselect, Options: opts()},
		{Key: "obligatorio", Label: "Obligatorio", Scope: ScopeClient, Type: TypeText, Required: true},
		{Key: "viejo", Label: "Viejo", Scope: ScopeClient, Type: TypeText, Archived: true},
		{Key: "cita_nota", Label: "Nota de cita", Scope: ScopeAppointment, Type: TypeText},
		{Key: "viejo_req", Label: "Viejo obligatorio", Scope: ScopeClient, Type: TypeText, Required: true, Archived: true},
		{Key: "cita_req", Label: "Obligatorio de cita", Scope: ScopeAppointment, Type: TypeText, Required: true},
	}
}

// create validates request as a new client record that already satisfies the required key.
func create(t *testing.T, request string) (string, []problem.FieldError) {
	t.Helper()
	got, errs := validateValues(defs(), ScopeClient, nil, json.RawMessage(request), true)
	return string(got), errs
}

func wantOneError(t *testing.T, errs []problem.FieldError, field, message string) {
	t.Helper()
	if len(errs) != 1 || errs[0].Field != field || errs[0].Message != message {
		t.Errorf("errors = %+v, want one on %q with %q", errs, field, message)
	}
}

func TestValidateAcceptedValues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"text", `{"nota":"Café negro","obligatorio":"x"}`, `{"nota":"Café negro","obligatorio":"x"}`},
		{"text trimmed", `{"nota":" hola ","obligatorio":"x"}`, `{"nota":"hola","obligatorio":"x"}`},
		{"text of exactly 500 runes", fmt.Sprintf(`{"nota":%q,"obligatorio":"x"}`, strings.Repeat("ñ", 500)),
			fmt.Sprintf(`{"nota":%q,"obligatorio":"x"}`, strings.Repeat("ñ", 500))},
		{"number", `{"peso":12.5,"obligatorio":"x"}`, `{"obligatorio":"x","peso":12.5}`},
		{"date", `{"cumple":"2026-10-24","obligatorio":"x"}`, `{"cumple":"2026-10-24","obligatorio":"x"}`},
		{"select", fmt.Sprintf(`{"color":%q,"obligatorio":"x"}`, optA), fmt.Sprintf(`{"color":%q,"obligatorio":"x"}`, optA)},
		{"multiselect", fmt.Sprintf(`{"gustos":[%q,%q],"obligatorio":"x"}`, optA, optB),
			fmt.Sprintf(`{"gustos":[%q,%q],"obligatorio":"x"}`, optA, optB)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errs := create(t, tt.body)
			if len(errs) != 0 || got != tt.want {
				t.Errorf("got %s, %+v; want %s and no errors", got, errs, tt.want)
			}
		})
	}
}

func TestValidateRejectedValues(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		field string
		want  string
	}{
		{"text of 501 runes", fmt.Sprintf(`{"nota":%q,"obligatorio":"x"}`, strings.Repeat("ñ", 501)), "attrs.nota", fmt.Sprintf(problem.MsgAttrTextInvalid, "Nota")},
		{"text not a string", `{"nota":5,"obligatorio":"x"}`, "attrs.nota", fmt.Sprintf(problem.MsgAttrTextInvalid, "Nota")},
		{"number as string", `{"peso":"12","obligatorio":"x"}`, "attrs.peso", fmt.Sprintf(problem.MsgAttrNumberInvalid, "Peso")},
		{"number out of range", `{"peso":1e999,"obligatorio":"x"}`, "attrs.peso", fmt.Sprintf(problem.MsgAttrNumberInvalid, "Peso")},
		{"date with time", `{"cumple":"2026-10-24T10:00:00Z","obligatorio":"x"}`, "attrs.cumple", fmt.Sprintf(problem.MsgAttrDateInvalid, "Cumpleaños")},
		{"date unpadded", `{"cumple":"2026-2-3","obligatorio":"x"}`, "attrs.cumple", fmt.Sprintf(problem.MsgAttrDateInvalid, "Cumpleaños")},
		{"option of another attribute", fmt.Sprintf(`{"color":%q,"obligatorio":"x"}`, optOth), "attrs.color", fmt.Sprintf(problem.MsgAttrOptionInvalid, "Color")},
		{"select not a string", `{"color":3,"obligatorio":"x"}`, "attrs.color", fmt.Sprintf(problem.MsgAttrOptionInvalid, "Color")},
		{"repeated id", fmt.Sprintf(`{"gustos":[%q,%q],"obligatorio":"x"}`, optA, optA), "attrs.gustos", fmt.Sprintf(problem.MsgAttrOptionRepeated, "Gustos")},
		{"repeated id in another case", fmt.Sprintf(`{"gustos":[%q,%q],"obligatorio":"x"}`, optA, strings.ToUpper(optA.String())), "attrs.gustos", fmt.Sprintf(problem.MsgAttrOptionRepeated, "Gustos")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := create(t, tt.body)
			wantOneError(t, errs, tt.field, tt.want)
		})
	}
}

func TestValidateRejectedWrites(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		field string
		want  string
	}{
		{"unknown key", `{"nope":1,"obligatorio":"x"}`, "attrs.nope", fmt.Sprintf(problem.MsgAttrUnknown, "nope")},
		{"other scope", `{"cita_nota":"x","obligatorio":"x"}`, "attrs.cita_nota", fmt.Sprintf(problem.MsgAttrOtherScope, "Nota de cita")},
		{"archived attribute", `{"viejo":"x","obligatorio":"x"}`, "attrs.viejo", fmt.Sprintf(problem.MsgAttrArchived, "Viejo")},
		{"archived option", fmt.Sprintf(`{"color":%q,"obligatorio":"x"}`, optOld), "attrs.color", fmt.Sprintf(problem.MsgAttrOptionArchived, "Color")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := create(t, tt.body)
			wantOneError(t, errs, tt.field, tt.want)
		})
	}
}

func TestValidateEmptyValues(t *testing.T) {
	required := fmt.Sprintf(problem.MsgAttrRequired, "Obligatorio")
	for _, empty := range []string{`""`, `"   "`} {
		got, errs := create(t, `{"nota":`+empty+`,"obligatorio":"x"}`)
		if len(errs) != 0 || got != `{"obligatorio":"x"}` {
			t.Errorf("create text %s = %s, %+v; want the key removed", empty, got, errs)
		}
		_, errs = create(t, `{"obligatorio":`+empty+`}`)
		wantOneError(t, errs, "attrs.obligatorio", required)
	}
	got, errs := create(t, `{"gustos":[],"obligatorio":"x"}`)
	if len(errs) != 0 || got != `{"obligatorio":"x"}` {
		t.Errorf("create [] = %s, %+v; want the key removed", got, errs)
	}
	got, errs = create(t, `{"nota":null,"obligatorio":"x"}`)
	if len(errs) != 0 || got != `{"obligatorio":"x"}` {
		t.Errorf("create null = %s, %+v; want it ignored", got, errs)
	}
	stored := json.RawMessage(`{"nota":"hola","obligatorio":"x"}`)
	out, errs := validateValues(defs(), ScopeClient, stored, json.RawMessage(`{"nota":null}`), false)
	if len(errs) != 0 || string(out) != `{"obligatorio":"x"}` {
		t.Errorf("update null = %s, %+v; want the key removed", out, errs)
	}
	_, errs = validateValues(defs(), ScopeClient, stored, json.RawMessage(`{"obligatorio":null}`), false)
	wantOneError(t, errs, "attrs.obligatorio", required)
	_, errs = validateValues(defs(), ScopeClient, stored, json.RawMessage(`{"gustos":[]}`), false)
	if len(errs) != 0 {
		t.Errorf("update with [] on an optional multiselect: %+v", errs)
	}
}

func TestValidateRequiredOnCreateOnly(t *testing.T) {
	_, errs := create(t, `{"nota":"x"}`)
	wantOneError(t, errs, "attrs.obligatorio", fmt.Sprintf(problem.MsgAttrRequired, "Obligatorio"))
	_, errs = create(t, ``)
	wantOneError(t, errs, "attrs.obligatorio", fmt.Sprintf(problem.MsgAttrRequired, "Obligatorio"))

	stored := json.RawMessage(`{"nota":"vieja"}`)
	out, errs := validateValues(defs(), ScopeClient, stored, json.RawMessage(`{"nota":"nueva"}`), false)
	if len(errs) != 0 || string(out) != `{"nota":"nueva"}` {
		t.Errorf("update without the required key = %s, %+v; want it accepted", out, errs)
	}
}

func TestValidateUnchangedArchivedValueIsAccepted(t *testing.T) {
	stored := json.RawMessage(fmt.Sprintf(`{"color":%q,"viejo":"vale","obligatorio":"x"}`, optOld))
	request := json.RawMessage(fmt.Sprintf(`{ "color": %q, "viejo": "vale", "nota": "hola" }`, optOld))
	out, errs := validateValues(defs(), ScopeClient, stored, request, false)
	if len(errs) != 0 {
		t.Fatalf("errors = %+v, want none", errs)
	}
	want := fmt.Sprintf(`{"color":%q,"nota":"hola","obligatorio":"x","viejo":"vale"}`, optOld)
	if string(out) != want {
		t.Errorf("merged = %s, want %s", out, want)
	}
}

func TestValidateNewArchivedOptionIsRejected(t *testing.T) {
	stored := json.RawMessage(fmt.Sprintf(`{"gustos":[%q],"obligatorio":"x"}`, optOld))
	keep := json.RawMessage(fmt.Sprintf(`{"gustos":[%q,%q]}`, optOld, optA))
	out, errs := validateValues(defs(), ScopeClient, stored, keep, false)
	if len(errs) != 0 {
		t.Fatalf("keeping the stored archived ID: %+v", errs)
	}
	if want := fmt.Sprintf(`{"gustos":[%q,%q],"obligatorio":"x"}`, optOld, optA); string(out) != want {
		t.Errorf("merged = %s, want %s", out, want)
	}
	fresh := json.RawMessage(fmt.Sprintf(`{"gustos":[%q]}`, optOld))
	_, errs = validateValues(defs(), ScopeClient, json.RawMessage(`{"obligatorio":"x"}`), fresh, false)
	wantOneError(t, errs, "attrs.gustos", fmt.Sprintf(problem.MsgAttrOptionArchived, "Gustos"))
}

func TestValidateAttrsNotObject(t *testing.T) {
	for _, bad := range []string{`[]`, `"x"`, `null`, `5`} {
		_, errs := validateValues(defs(), ScopeClient, nil, json.RawMessage(bad), true)
		wantOneError(t, errs, "attrs", problem.MsgAttrsInvalid)
		_, errs = validateValues(defs(), ScopeClient, json.RawMessage(bad), nil, false)
		wantOneError(t, errs, "attrs", problem.MsgAttrsInvalid)
	}
}

func TestValidateMergeKeepsUntouchedKeys(t *testing.T) {
	stored := json.RawMessage(`{"nota":"a","peso":70,"obligatorio":"x"}`)
	out, errs := validateValues(defs(), ScopeClient, stored, json.RawMessage(`{"peso":71.5}`), false)
	if len(errs) != 0 || string(out) != `{"nota":"a","obligatorio":"x","peso":71.5}` {
		t.Errorf("merged = %s, %+v", out, errs)
	}
}

func TestValidateErrorsInKeyOrder(t *testing.T) {
	_, errs := create(t, `{"peso":"x","cumple":"y","obligatorio":"x"}`)
	if len(errs) != 2 || errs[0].Field != "attrs.cumple" || errs[1].Field != "attrs.peso" {
		t.Errorf("errors = %+v, want cumple then peso", errs)
	}
}

func TestValidateNullOnArchivedAttributeIsRejected(t *testing.T) {
	stored := json.RawMessage(`{"viejo":"vale","obligatorio":"x"}`)
	_, errs := validateValues(defs(), ScopeClient, stored, json.RawMessage(`{"viejo":null}`), false)
	wantOneError(t, errs, "attrs.viejo", fmt.Sprintf(problem.MsgAttrArchived, "Viejo"))
}

func TestValidateRequiredIgnoresArchivedAndOtherScope(t *testing.T) {
	got, errs := create(t, `{"obligatorio":"x"}`)
	if len(errs) != 0 || got != `{"obligatorio":"x"}` {
		t.Errorf("got %s, %+v; want no error for an archived or other-scope required attribute", got, errs)
	}
}

func TestValidateUppercaseOptionIDsMatchStored(t *testing.T) {
	stored := json.RawMessage(fmt.Sprintf(`{"color":%q,"gustos":[%q],"obligatorio":"x"}`, optOld, optOld))
	request := json.RawMessage(fmt.Sprintf(`{"color":%q,"gustos":[%q]}`, strings.ToUpper(optOld.String()), strings.ToUpper(optOld.String())))
	out, errs := validateValues(defs(), ScopeClient, stored, request, false)
	if len(errs) != 0 || string(out) != string(stored) {
		t.Errorf("got %s, %+v; want the stored value kept without errors", out, errs)
	}
}

// wantOneValidation checks that err is a validation problem with exactly one field error.
func wantOneValidation(t *testing.T, err error, field, message string) {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Errorf("error = %#v, want a validation problem", err)
		return
	}
	wantOneError(t, v.Errors, field, message)
}
