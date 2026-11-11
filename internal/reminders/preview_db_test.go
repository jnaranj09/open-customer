//go:build db

package reminders

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// previewAttribute adds an attribute and returns it.
func (e dueEnv) previewAttribute(t *testing.T, label string, scope attributes.Scope, typ attributes.Type) attributes.Attribute {
	t.Helper()
	a, err := e.m.attributes.Create(t.Context(), e.pool, attributes.AttributeInput{Label: label, Scope: scope, Type: typ})
	if err != nil {
		t.Fatalf("create attribute %s: %v", label, err)
	}
	return a
}

func (e dueEnv) preview(t *testing.T, body string, clientID uuid.NullUUID) Preview {
	t.Helper()
	p, err := e.m.Preview(t.Context(), e.pool, body, clientID)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	return p
}

func TestPreviewSampleShowsPlaceholders(t *testing.T) {
	e := newDueEnv(t)
	e.previewAttribute(t, "Bebida favorita", attributes.ScopeClient, attributes.TypeText)
	form := e.previewAttribute(t, "Forma", attributes.ScopeAppointment, attributes.TypeSelect)
	if _, err := e.m.attributes.AddOption(t.Context(), e.pool, form.ID, attributes.OptionInput{Label: "Almendra"}); err != nil {
		t.Fatalf("add option: %v", err)
	}
	got := e.preview(t, "{{client.first_name}}: {{client.bebida_favorita}} {{appointment.forma}}", uuid.NullUUID{})
	if got.Text != "Ana: [Bebida favorita] Almendra" || len(got.UnknownVars) != 0 {
		t.Fatalf("Preview = %+v; want the placeholders and no unknown variables", got)
	}
}

func TestPreviewRealClientUsesLatestActiveAppointment(t *testing.T) {
	e := newDueEnv(t)
	manicure, pedicure := e.service(t, "Manicura", 60, nil), e.service(t, "Pedicura", 60, nil)
	ana := e.client(t, "Ana Rojas", false)
	e.book(t, ana.ID, e.at(2026, 10, 5, 9, 0), manicure)
	e.book(t, ana.ID, e.at(2026, 10, 10, 9, 0), pedicure)
	cancelled := e.book(t, ana.ID, e.at(2026, 10, 20, 9, 0), manicure)
	if _, err := e.appts.SetStatus(t.Context(), e.pool, cancelled.ID, appointments.Cancelled, false); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	got := e.preview(t, "{{client.name}} {{appointment.services}} {{appointment.date}}", uuid.NullUUID{UUID: ana.ID, Valid: true})
	if want := "Ana Rojas Pedicura sábado 10 de octubre"; got.Text != want {
		t.Fatalf("Text = %q; want %q", got.Text, want)
	}
}

func TestPreviewClientWithoutAppointmentUsesSample(t *testing.T) {
	e := newDueEnv(t)
	ana := e.client(t, "Ana Rojas", false)
	got := e.preview(t, "{{client.name}} {{appointment.services}}", uuid.NullUUID{UUID: ana.ID, Valid: true})
	if got.Text != "Ana Rojas Servicio de ejemplo" {
		t.Fatalf("Text = %q; want the client with the sample appointment", got.Text)
	}
}

func TestPreviewArchivedClientIsAllowed(t *testing.T) {
	e := newDueEnv(t)
	ana := e.client(t, "Ana Rojas", false)
	archived := true
	if _, err := e.clients.Update(t.Context(), e.pool, ana.ID, clients.ClientPatch{Archived: &archived}, "ES"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got := e.preview(t, "{{client.name}}", uuid.NullUUID{UUID: ana.ID, Valid: true})
	if got.Text != "Ana Rojas" {
		t.Fatalf("Text = %q; want the archived client's name", got.Text)
	}
}

func TestPreviewUnknownVarsDeduplicated(t *testing.T) {
	e := newDueEnv(t)
	got := e.preview(t, "{{client.x}} {{client.x}} {{appointment.y}}", uuid.NullUUID{})
	if want := []string{"client.x", "appointment.y"}; !reflect.DeepEqual(got.UnknownVars, want) {
		t.Fatalf("UnknownVars = %v; want %v", got.UnknownVars, want)
	}
}

func TestPreviewNoUnknownVarsIsEmptyNotNil(t *testing.T) {
	e := newDueEnv(t)
	got := e.preview(t, "Hola", uuid.NullUUID{})
	if got.UnknownVars == nil || len(got.UnknownVars) != 0 {
		t.Fatalf("UnknownVars = %#v; want an empty, non-nil list", got.UnknownVars)
	}
}

func TestPreviewUnknownClientIs422(t *testing.T) {
	e := newDueEnv(t)
	_, err := e.m.Preview(t.Context(), e.pool, "Hola", uuid.NullUUID{UUID: uuid.New(), Valid: true})
	want := problem.FieldError{Field: "client_id", Message: problem.MsgClientUnavailable}
	if got := fieldsOf(t, err); len(got) != 1 || got[0] != want {
		t.Fatalf("errors = %+v; want [%+v]", got, want)
	}
}

func TestPreviewBadBraceRendersLiterally(t *testing.T) {
	e := newDueEnv(t)
	got := e.preview(t, "Hola {{", uuid.NullUUID{})
	if got.Text != "Hola {{" {
		t.Fatalf("Text = %q; want %q", got.Text, "Hola {{")
	}
}

func TestPreviewBadBraceKeepsCheckingOtherVariables(t *testing.T) {
	e := newDueEnv(t)
	got := e.preview(t, "{{ {{client.x}}", uuid.NullUUID{})
	if want := []string{"client.x"}; !reflect.DeepEqual(got.UnknownVars, want) {
		t.Fatalf("UnknownVars = %v; want %v", got.UnknownVars, want)
	}
}

func TestPreviewBeforeSetupIsNotFound(t *testing.T) {
	m := New(Options{Clock: clock.NewFake(start), Attributes: attributes.New(attributes.Options{})})
	_, err := m.Preview(t.Context(), dbtest.Pool(t), "Hola", uuid.NullUUID{})
	if !errors.As(err, &problem.NotFound{}) {
		t.Fatalf("err = %v; want problem.NotFound", err)
	}
}

func TestPreviewDBErrors(t *testing.T) {
	for name, withClient := range map[string]bool{"sample": false, "client": true} {
		t.Run(name, func(t *testing.T) {
			sweepDBErrors(t, func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
				t.Helper()
				e := newDueEnv(t)
				var id uuid.NullUUID
				if withClient {
					ana := e.client(t, "Ana Rojas", false)
					e.book(t, ana.ID, e.at(2026, 10, 5, 9, 0), e.service(t, "Manicura", 60, nil))
					id = uuid.NullUUID{UUID: ana.ID, Valid: true}
				}
				return e.pool, func(q db.DBTX) error { _, err := e.m.Preview(t.Context(), q, "Hola", id); return err }
			})
		})
	}
}

// The rule preview (reminders-and-replies design, "Look-ahead pass", CUJ R7; item 42). The clock
// starts on 30 Sep, so each visit is booked then; "today" is 2 Nov.

func (e dueEnv) rulePreview(t *testing.T, draft RuleDraft) RulePreviewResult {
	t.Helper()
	out, err := e.m.RulePreview(t.Context(), e.pool, e.at(2026, 11, 2, 9, 0), draft)
	if err != nil {
		t.Fatalf("RulePreview: %v", err)
	}
	return out
}

// visitOn books a client's 10:00 visit of svc on the given day.
func (e dueEnv) visitOn(t *testing.T, name string, m time.Month, d int, svc uuid.UUID) clients.Client {
	t.Helper()
	c := e.client(t, name, true)
	e.book(t, c.ID, e.at(2026, m, d, 10, 0), svc)
	return c
}

func wantNext(t *testing.T, got *NextClient, name string, due time.Time) {
	t.Helper()
	if got == nil || got.ClientName != name || !got.DueDate.Equal(due) {
		t.Fatalf("next = %+v, want %s on %s", got, name, due.Format(time.DateOnly))
	}
}

func TestCUJR7PreviewCountsTodayAndFindsTheNext(t *testing.T) {
	e := newDueEnv(t)
	acrilico := e.service(t, "Acrílico", 60, nil)
	g := e.general(t, 30)
	for _, name := range []string{"Bea", "Dora", "Eva", "Fina"} {
		e.visitOn(t, name, 10, 12, acrilico)
	}
	e.visitOn(t, "Carla", 10, 20, acrilico)
	e.visitOn(t, "Ana", 10, 3, acrilico) // due 24 Oct with 21 days: late, not counted

	got := e.rulePreview(t, RuleDraft{RuleID: nu(g.ID), OffsetDays: 21, TemplateID: g.TemplateID})
	if got.TodayCount != 4 {
		t.Fatalf("today_count = %d, want 4", got.TodayCount)
	}
	wantNext(t, got.Next, "Carla", e.at(2026, 11, 10, 0, 0))

	rules, err := e.m.ListRules(t.Context(), e.pool)
	if err != nil || len(rules) != 1 || rules[0].OffsetDays != 30 {
		t.Fatalf("rules = %+v, %v; want the saved 30-day rule unchanged", rules, err)
	}
}

func TestPreviewDraftReplacesTheSavedRule(t *testing.T) {
	e := newDueEnv(t)
	acrilico := e.service(t, "Acrílico", 60, nil)
	g := e.general(t, 21)
	e.visitOn(t, "Ana", 10, 12, acrilico) // the saved 21 days are due today

	got := e.rulePreview(t, RuleDraft{RuleID: nu(g.ID), OffsetDays: 30, TemplateID: g.TemplateID})
	if got.TodayCount != 0 {
		t.Fatalf("today_count = %d, want 0: the 30-day draft is due 11 Nov", got.TodayCount)
	}
	wantNext(t, got.Next, "Ana", e.at(2026, 11, 11, 0, 0))
}

func TestPreviewDraftOffsetBeyondTheSavedRulesFindsItsVisits(t *testing.T) {
	e := newDueEnv(t)
	acrilico := e.service(t, "Acrílico", 60, nil)
	g := e.general(t, 30)
	e.visitOn(t, "Ana", 4, 16, acrilico) // 200 days before 2 Nov

	got := e.rulePreview(t, RuleDraft{OffsetDays: 200, TemplateID: g.TemplateID})
	if got.TodayCount != 1 {
		t.Fatalf("today_count = %d, want Ana's visit 200 days ago", got.TodayCount)
	}
}

func TestPreviewNewSpecificRule(t *testing.T) {
	e := newDueEnv(t)
	acrilico, cejas := e.service(t, "Acrílico", 60, nil), e.service(t, "Cejas", 30, nil)
	tpl := e.template(t, "Uñas", "Hola {{client.first_name}}")
	e.visitOn(t, "Ana", 10, 12, acrilico)
	e.visitOn(t, "Bea", 10, 12, cejas)
	e.visitOn(t, "Carla", 10, 15, acrilico)

	got := e.rulePreview(t, RuleDraft{ServiceID: nu(acrilico), OffsetDays: 21, TemplateID: tpl})
	if got.TodayCount != 1 {
		t.Fatalf("today_count = %d, want Ana only", got.TodayCount)
	}
	wantNext(t, got.Next, "Carla", e.at(2026, 11, 5, 0, 0))
}

func TestPreviewWithNothingAheadHasNoNext(t *testing.T) {
	e := newDueEnv(t)
	g := e.general(t, 30)
	if got := e.rulePreview(t, RuleDraft{RuleID: nu(g.ID), OffsetDays: 30, TemplateID: g.TemplateID}); got.TodayCount != 0 || got.Next != nil {
		t.Fatalf("preview = %+v, want 0 and no next", got)
	}
}

func TestPreviewRejects(t *testing.T) {
	e := newDueEnv(t)
	tpl := e.template(t, "Aviso", "Hola")
	before := e.rule(t, RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: tpl})
	cases := []struct {
		name  string
		draft RuleDraft
		field string
	}{
		{"a before rule is 422 on rule_id", RuleDraft{RuleID: nu(before.ID), OffsetDays: 30, TemplateID: tpl}, "rule_id"},
		{"a come-back offset of 0 is 422", RuleDraft{OffsetDays: 0, TemplateID: tpl}, "offset_days"},
		{"an unknown template is 422", RuleDraft{OffsetDays: 30, TemplateID: uuid.New()}, "template_id"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := e.m.RulePreview(t.Context(), e.pool, e.at(2026, 11, 2, 9, 0), c.draft)
			var v problem.Validation
			if !errors.As(err, &v) || len(v.Errors) != 1 || v.Errors[0].Field != c.field {
				t.Fatalf("err = %v, want a 422 on %s", err, c.field)
			}
		})
	}
	t.Run("an unknown rule_id is not found", func(t *testing.T) {
		_, err := e.m.RulePreview(t.Context(), e.pool, e.at(2026, 11, 2, 9, 0), RuleDraft{RuleID: nu(uuid.New()), OffsetDays: 30, TemplateID: tpl})
		if !errors.As(err, new(problem.NotFound)) {
			t.Fatalf("err = %v, want NotFound", err)
		}
	})
}
