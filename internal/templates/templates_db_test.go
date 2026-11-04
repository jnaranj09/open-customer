//go:build db

package templates

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var start = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

type fixture struct {
	pool  *pgxpool.Pool
	clock *clock.Fake
	attrs *attributes.Module
	m     *Module
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	fake := clock.NewFake(start)
	attrs := attributes.New(attributes.Options{Clock: fake})
	return fixture{pool: dbtest.Pool(t), clock: fake, attrs: attrs, m: New(Options{Clock: fake, Attributes: attrs})}
}

func (f fixture) attribute(t *testing.T, label string, scope attributes.Scope) attributes.Attribute {
	t.Helper()
	a, err := f.attrs.Create(t.Context(), f.pool, attributes.AttributeInput{Label: label, Scope: scope, Type: attributes.TypeText})
	if err != nil {
		t.Fatalf("create attribute %q: %v", label, err)
	}
	return a
}

func (f fixture) archiveAttribute(t *testing.T, a attributes.Attribute) {
	t.Helper()
	archived := true
	if _, err := f.attrs.Update(t.Context(), f.pool, a.ID, attributes.AttributePatch{Archived: &archived}); err != nil {
		t.Fatalf("archive attribute: %v", err)
	}
}

func (f fixture) create(t *testing.T, name, body string) Template {
	t.Helper()
	tpl, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: name, Body: body})
	if err != nil {
		t.Fatalf("Create %q: %v", name, err)
	}
	return tpl
}

func fieldsOf(t *testing.T, err error) []problem.FieldError {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, want problem.Validation", err)
	}
	return v.Errors
}

func unknownVar(key string) problem.FieldError {
	return problem.FieldError{Field: "body", Message: fmt.Sprintf(problem.MsgTemplateUnknownVar, key)}
}

func TestCreateTemplateOK(t *testing.T) {
	f := newFixture(t)
	f.attribute(t, "Bebida favorita", attributes.ScopeClient)
	body := `  Hola {{client.first_name}}, ¿un {{client.bebida_favorita | "café"}}?  `
	got, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "  Recordatorio ", Body: body})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID == uuid.Nil || got.Name != "Recordatorio" || got.Body != body || got.Archived {
		t.Errorf("Create = %+v, want a trimmed name, the body as sent, not archived", got)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0] != got {
		t.Errorf("List = %+v, want [%+v]", list, got)
	}
}

func TestListIsEmptyNotNilAndOrderedByNameThenID(t *testing.T) {
	f := newFixture(t)
	empty, err := f.m.List(t.Context(), f.pool)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("List on an empty table = %#v, %v; want an empty non-nil slice", empty, err)
	}
	b := f.create(t, "Beta", "uno")
	a := f.create(t, "Alfa", "dos")
	got, err := f.m.List(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID {
		t.Errorf("List = %+v, want Alfa then Beta", got)
	}
}

func TestCreateUnknownVarNamesToken(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: "Hola {{client.bebida_favorita}}"})
	errs := fieldsOf(t, err)
	if len(errs) != 1 || errs[0] != unknownVar("bebida_favorita") {
		t.Fatalf("errors = %+v, want the unknown variable bebida_favorita", errs)
	}
	if errs[0].Message != "La variable 'bebida_favorita' no existe" {
		t.Errorf("message = %q", errs[0].Message)
	}
}

func TestCreateWrongScopeVarIsUnknown(t *testing.T) {
	f := newFixture(t)
	f.attribute(t, "Bebida", attributes.ScopeClient)
	_, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: "{{appointment.bebida}}"})
	errs := fieldsOf(t, err)
	if len(errs) != 1 || errs[0] != unknownVar("bebida") {
		t.Fatalf("errors = %+v, want the unknown variable bebida", errs)
	}
}

func TestSaveWithArchivedAttributeSucceeds(t *testing.T) {
	f := newFixture(t)
	a := f.attribute(t, "Bebida", attributes.ScopeClient)
	f.archiveAttribute(t, a)
	if _, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: "{{client.bebida}}"}); err != nil {
		t.Errorf("Create with an archived attribute: %v", err)
	}
	tpl := f.create(t, "Y", "hola")
	if _, err := f.m.Update(t.Context(), f.pool, tpl.ID, TemplatePatch{Body: ptr("{{client.bebida}}")}); err != nil {
		t.Errorf("Update with an archived attribute: %v", err)
	}
}

func TestCreateUnclosedIsValidation(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: "Hola {{client.name"})
	errs := fieldsOf(t, err)
	if len(errs) != 1 || errs[0] != unclosed(6) {
		t.Fatalf("errors = %+v, want one unclosed error at position 6", errs)
	}
}

func TestCreateReportsParseErrorsBeforeUnknownVars(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: "{{client.nope}} {{"})
	errs := fieldsOf(t, err)
	want := []problem.FieldError{unclosed(17), unknownVar("nope")}
	if len(errs) != 2 || errs[0] != want[0] || errs[1] != want[1] {
		t.Fatalf("errors = %+v, want %+v", errs, want)
	}
}

func TestCreateReportsUnknownVarsInBodyOrder(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: "{{client.zeta}} {{client.alfa}}"})
	errs := fieldsOf(t, err)
	if len(errs) != 2 || errs[0] != unknownVar("zeta") || errs[1] != unknownVar("alfa") {
		t.Fatalf("errors = %+v, want zeta then alfa", errs)
	}
}

func TestCreateCollectsNameBodyParseAndUnknownErrors(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: " ", Body: "{{client.nope}} {{"})
	errs := fieldsOf(t, err)
	want := []problem.FieldError{{Field: "name", Message: problem.MsgNameInvalid}, unclosed(17), unknownVar("nope")}
	if len(errs) != 3 || errs[0] != want[0] || errs[1] != want[1] || errs[2] != want[2] {
		t.Fatalf("errors = %+v, want %+v", errs, want)
	}
}

func TestCreateBlankBodyIs422(t *testing.T) {
	f := newFixture(t)
	for name, body := range map[string]string{"empty": "", "spaces": "  \n "} {
		t.Run(name, func(t *testing.T) {
			_, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: body})
			errs := fieldsOf(t, err)
			if len(errs) != 1 || errs[0] != (problem.FieldError{Field: "body", Message: problem.MsgTemplateBodyInvalid}) {
				t.Errorf("errors = %+v, want the body error", errs)
			}
		})
	}
}

func TestBodyLengthLimitCountsRunes(t *testing.T) {
	f := newFixture(t)
	if _, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: strings.Repeat("ñ", 2000)}); err != nil {
		t.Errorf("2000 runes: %v", err)
	}
	_, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: strings.Repeat("ñ", 2001)})
	errs := fieldsOf(t, err)
	if len(errs) != 1 || errs[0] != (problem.FieldError{Field: "body", Message: problem.MsgTemplateBodyInvalid}) {
		t.Errorf("2001 runes: errors = %+v, want the body error", errs)
	}
}

func TestBodyKeepsItsSurroundingSpace(t *testing.T) {
	f := newFixture(t)
	body := strings.Repeat("a", 1999) + " "
	got, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: body})
	if err != nil || got.Body != body {
		t.Errorf("Create = %q, %v; want the body unchanged", got.Body, err)
	}
	_, err = f.m.Create(t.Context(), f.pool, TemplateInput{Name: "X", Body: body + " "})
	if errs := fieldsOf(t, err); len(errs) != 1 || errs[0].Field != "body" {
		t.Errorf("body of 2001 runes with a trailing space: errors = %+v, want the body error", errs)
	}
}

func TestCreateNameLimits(t *testing.T) {
	f := newFixture(t)
	if _, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: strings.Repeat("ñ", 80), Body: "hola"}); err != nil {
		t.Errorf("80 runes: %v", err)
	}
	for name, n := range map[string]string{"empty": "  ", "81 runes": strings.Repeat("ñ", 81)} {
		t.Run(name, func(t *testing.T) {
			_, err := f.m.Create(t.Context(), f.pool, TemplateInput{Name: n, Body: "hola"})
			errs := fieldsOf(t, err)
			if len(errs) != 1 || errs[0] != (problem.FieldError{Field: "name", Message: problem.MsgNameInvalid}) {
				t.Errorf("errors = %+v, want the name error", errs)
			}
		})
	}
}

func TestUpdateArchive(t *testing.T) {
	f := newFixture(t)
	tpl := f.create(t, "Uno", "hola")
	got, err := f.m.Update(t.Context(), f.pool, tpl.ID, TemplatePatch{Archived: &yes})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !got.Archived || got.Name != "Uno" || got.Body != "hola" {
		t.Errorf("archived = %+v, want archived with name and body kept", got)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil || len(list) != 1 || !list[0].Archived {
		t.Errorf("List = %+v, %v; want the archived template still listed", list, err)
	}
	got, err = f.m.Update(t.Context(), f.pool, tpl.ID, TemplatePatch{Archived: &no})
	if err != nil || got.Archived {
		t.Errorf("restore = %+v, %v; want not archived", got, err)
	}
}

func TestUpdateChangesNameAndBody(t *testing.T) {
	f := newFixture(t)
	tpl := f.create(t, "Uno", "hola")
	got, err := f.m.Update(t.Context(), f.pool, tpl.ID, TemplatePatch{Name: ptr(" Dos "), Body: ptr("adiós")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.ID != tpl.ID || got.Name != "Dos" || got.Body != "adiós" {
		t.Errorf("Update = %+v, want name Dos and body adiós", got)
	}
	list, _ := f.m.List(t.Context(), f.pool)
	if len(list) != 1 || list[0] != got {
		t.Errorf("List = %+v, want the stored change %+v", list, got)
	}
}

func TestUpdateWithoutBodyDoesNotRunTheSaveCheck(t *testing.T) {
	f := newFixture(t)
	f.attribute(t, "Bebida", attributes.ScopeClient)
	tpl := f.create(t, "Uno", "{{client.bebida}}")
	// Store a body the check would reject, to show a rename does not run the check.
	if _, err := f.pool.Exec(t.Context(), `UPDATE message_templates SET body = '{{client.nope}}' WHERE id = $1`, tpl.ID); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := f.m.Update(t.Context(), f.pool, tpl.ID, TemplatePatch{Name: ptr("Dos")})
	if err != nil || got.Name != "Dos" || got.Body != "{{client.nope}}" {
		t.Errorf("Update = %+v, %v; want the rename to succeed and the body kept", got, err)
	}
}

func TestUpdateChecksTheBodyWhenSent(t *testing.T) {
	f := newFixture(t)
	tpl := f.create(t, "Uno", "hola")
	_, err := f.m.Update(t.Context(), f.pool, tpl.ID, TemplatePatch{Body: ptr("{{client.nope}} {{")})
	errs := fieldsOf(t, err)
	if len(errs) != 2 || errs[0] != unclosed(17) || errs[1] != unknownVar("nope") {
		t.Errorf("errors = %+v, want unclosed then unknown", errs)
	}
	_, err = f.m.Update(t.Context(), f.pool, tpl.ID, TemplatePatch{Name: ptr(" ")})
	if errs := fieldsOf(t, err); len(errs) != 1 || errs[0].Field != "name" {
		t.Errorf("errors = %+v, want the name error", errs)
	}
	_, err = f.m.Update(t.Context(), f.pool, tpl.ID, TemplatePatch{Body: ptr(" ")})
	if errs := fieldsOf(t, err); len(errs) != 1 || errs[0].Message != problem.MsgTemplateBodyInvalid {
		t.Errorf("errors = %+v, want the body error", errs)
	}
	list, _ := f.m.List(t.Context(), f.pool)
	if list[0].Body != "hola" || list[0].Name != "Uno" {
		t.Errorf("stored = %+v, want it unchanged after rejected updates", list[0])
	}
}

func TestUpdateUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Update(t.Context(), f.pool, uuid.New(), TemplatePatch{Name: ptr("X")})
	if !errors.Is(err, problem.NotFound{}) {
		t.Errorf("err = %v, want problem.NotFound", err)
	}
}

func TestVarsListsActiveOnlyInOrder(t *testing.T) {
	f := newFixture(t)
	es, err := locale.For("es")
	if err != nil {
		t.Fatalf("locale: %v", err)
	}
	f.attribute(t, "Bebida favorita", attributes.ScopeClient)
	gone := f.attribute(t, "Zona", attributes.ScopeClient)
	f.attribute(t, "Sala", attributes.ScopeAppointment)
	f.archiveAttribute(t, gone)
	renamed := f.attribute(t, "Alergia", attributes.ScopeClient)
	label := "Alergias conocidas"
	if _, err := f.attrs.Update(t.Context(), f.pool, renamed.ID, attributes.AttributePatch{Label: &label}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	got, err := f.m.Vars(t.Context(), f.pool, es)
	if err != nil {
		t.Fatalf("Vars: %v", err)
	}
	want := []TemplateVar{
		{"client", "name", "Nombre"},
		{"client", "first_name", "Primer nombre"},
		{"client", "bebida_favorita", "Bebida favorita"},
		{"client", "alergia", "Alergias conocidas"},
		{"appointment", "date", "Fecha de la cita"},
		{"appointment", "time", "Hora de la cita"},
		{"appointment", "services", "Servicios"},
		{"appointment", "sala", "Sala"},
		{"business", "name", "Nombre del negocio"},
	}
	if len(got) != len(want) {
		t.Fatalf("Vars = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Vars[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

var yes, no = true, false
