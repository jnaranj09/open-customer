//go:build db

package reminders

import (
	"errors"
	"reflect"
	"testing"

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
