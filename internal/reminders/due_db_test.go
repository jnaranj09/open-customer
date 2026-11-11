//go:build db

package reminders

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nyaruka/phonenumbers"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/services"
	"github.com/jnaranj09/open-customer/internal/settings"
	"github.com/jnaranj09/open-customer/internal/templates"
)

// dueEnv is a migrated database with the real modules the due list reads from, all on one fake
// clock. Rows get created_at from the clock, so a test sets it before making them.
type dueEnv struct {
	pool      *pgxpool.Pool
	clock     *clock.Fake
	tz        *time.Location
	m         *Module
	services  *services.Module
	templates *templates.Module
	clients   *clients.Module
	appts     *appointments.Module
}

// newDueEnv sets the business up in Europe/Madrid, with the clock on 2026-09-30 10:00 local.
func newDueEnv(t *testing.T) dueEnv {
	t.Helper()
	pool := dbtest.Pool(t)
	tz := testZone(t)
	fake := clock.NewFake(time.Date(2026, 9, 30, 10, 0, 0, 0, tz))
	attrs := attributes.New(attributes.Options{Clock: fake})
	svcs := services.New(services.Options{Clock: fake})
	e := dueEnv{
		pool: pool, clock: fake, tz: tz,
		m:         New(Options{Clock: fake, Attributes: attrs}),
		services:  svcs,
		templates: templates.New(templates.Options{Clock: fake, Attributes: attrs}),
		clients:   clients.New(clients.Options{Clock: fake, Attributes: attrs}),
		appts:     appointments.New(appointments.Options{Clock: fake, Services: svcs, Attributes: attrs}),
	}
	if _, err := settings.New(settings.Options{Clock: fake}).Create(t.Context(), pool, settings.SettingsInput{
		Name: "Estudio Sol", Timezone: "Europe/Madrid", Locale: "es", Currency: "EUR", CountryCode: "ES",
		AccentColor: "#aa3366",
	}); err != nil {
		t.Fatalf("create settings: %v", err)
	}
	return e
}

// at is a business-local time.
func (e dueEnv) at(y int, m time.Month, d, h, minute int) time.Time {
	return time.Date(y, m, d, h, minute, 0, 0, e.tz)
}

// now is 2026-10-22 08:00 local, the day the 21-day come-back of a 2026-10-01 visit falls due.
func (e dueEnv) now() time.Time { return e.at(2026, 10, 22, 8, 0) }

func (e dueEnv) service(t *testing.T, name string, minutes int, price *int64) uuid.UUID {
	t.Helper()
	s, err := e.services.Create(t.Context(), e.pool, services.ServiceInput{Name: name, DurationMin: minutes, PriceCents: price, Color: "#aa3366"})
	if err != nil {
		t.Fatalf("create service %s: %v", name, err)
	}
	return s.ID
}

func (e dueEnv) template(t *testing.T, name, body string) uuid.UUID {
	t.Helper()
	tpl, err := e.templates.Create(t.Context(), e.pool, templates.TemplateInput{Name: name, Body: body})
	if err != nil {
		t.Fatalf("create template %s: %v", name, err)
	}
	return tpl.ID
}

// client creates a client; withPhone gives her the ES example number.
func (e dueEnv) client(t *testing.T, name string, withPhone bool) clients.Client {
	t.Helper()
	in := clients.ClientInput{Name: name, AllowDuplicatePhone: true}
	if withPhone {
		phone := phonenumbers.Format(phonenumbers.GetExampleNumber("ES"), phonenumbers.E164)
		in.Phone = &phone
	}
	c, err := e.clients.Create(t.Context(), e.pool, in, "ES")
	if err != nil {
		t.Fatalf("create client %s: %v", name, err)
	}
	return c
}

// book creates an appointment of client at starts, its end from the services' durations.
func (e dueEnv) book(t *testing.T, clientID uuid.UUID, starts time.Time, serviceIDs ...uuid.UUID) appointments.Appointment {
	t.Helper()
	a, err := e.appts.Create(t.Context(), e.pool, appointments.AppointmentInput{
		ClientID: clientID, StartsAt: starts, ServiceIDs: serviceIDs, AllowOverlap: true,
	})
	if err != nil {
		t.Fatalf("book appointment: %v", err)
	}
	return a
}

func (e dueEnv) rule(t *testing.T, in RuleInput) Rule {
	t.Helper()
	r, err := e.m.CreateRule(t.Context(), e.pool, in)
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	return r
}

func (e dueEnv) record(t *testing.T, entry LogEntry) LogItem {
	t.Helper()
	it, _, err := e.m.Record(t.Context(), e.pool, entry)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	return it
}

func (e dueEnv) due(t *testing.T, q db.DBTX) []DueItem {
	t.Helper()
	items, err := e.m.DueList(t.Context(), q, e.now())
	if err != nil {
		t.Fatalf("DueList: %v", err)
	}
	return items
}

// comeBackSetup is Ana Rojas's "Acrílico" visit on 2026-10-01 15:00 (booked 2026-09-30) and a
// 21-day come-back rule on "Acrílico" with return service "Relleno", made with "Incluir atrasados".
type comeBackSetup struct {
	acrilico, relleno, pedicure uuid.UUID
	ana                         clients.Client
	visit                       appointments.Appointment
	rule                        Rule
}

func newComeBack(t *testing.T, e dueEnv) comeBackSetup {
	t.Helper()
	price := int64(2500)
	s := comeBackSetup{
		acrilico: e.service(t, "Acrílico", 60, nil),
		relleno:  e.service(t, "Relleno", 45, nil),
		pedicure: e.service(t, "Pedicure", 30, &price),
		ana:      e.client(t, "Ana Rojas", true),
	}
	s.visit = e.book(t, s.ana.ID, e.at(2026, 10, 1, 15, 0), s.acrilico)
	s.rule = e.rule(t, RuleInput{
		Kind: KindComeBack, ServiceID: nu(s.acrilico), ReturnServiceID: nu(s.relleno), OffsetDays: 21,
		TemplateID: e.template(t, "Relleno", "Hola {{client.first_name}}, ¿agendamos tu relleno?"), IncludeOverdue: true,
	})
	return s
}

func TestDueListCancelLaterBookingBringsBackComeBack(t *testing.T) {
	e := newDueEnv(t)
	s := newComeBack(t, e)
	later := e.book(t, s.ana.ID, e.at(2026, 11, 12, 10, 0), s.relleno)
	e.clock.Set(e.now())
	none(t, e.due(t, e.pool))

	if _, err := e.appts.SetStatus(t.Context(), e.pool, later.ID, appointments.Cancelled, false); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	it := one(t, e.due(t, e.pool))
	if it.AppointmentID != s.visit.ID || it.Mode != ModeMessage {
		t.Fatalf("item = %+v; want the visit's come-back message", it)
	}
}

func TestDueListRemovingReturnServiceBringsBack(t *testing.T) {
	e := newDueEnv(t)
	s := newComeBack(t, e)
	later := e.book(t, s.ana.ID, e.at(2026, 11, 12, 10, 0), s.relleno, s.pedicure)
	e.clock.Set(e.now())
	none(t, e.due(t, e.pool))

	if _, err := e.appts.Update(t.Context(), e.pool, later.ID, appointments.AppointmentPatch{ServiceIDs: &[]uuid.UUID{s.pedicure}}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	it := one(t, e.due(t, e.pool))
	if it.AppointmentID != s.visit.ID {
		t.Fatalf("item appointment = %s; want the visit %s", it.AppointmentID, s.visit.ID)
	}
}

func TestDueListAgregarALaCitaRemovesPrompt(t *testing.T) {
	e := newDueEnv(t)
	s := newComeBack(t, e)
	later := e.book(t, s.ana.ID, e.at(2026, 11, 12, 10, 0), s.pedicure)
	e.clock.Set(e.now())
	it := one(t, e.due(t, e.pool))
	if it.Mode != ModeBookingPrompt || it.Prompt == nil || it.Prompt.AppointmentID != later.ID {
		t.Fatalf("item = %+v; want a booking prompt on %s", it, later.ID)
	}
	newPrice := int64(3000)
	if _, err := e.services.Update(t.Context(), e.pool, s.pedicure, services.ServicePatch{PriceCents: nullable.NewNullableWithValue(newPrice)}); err != nil {
		t.Fatalf("update service price: %v", err)
	}

	ids := []uuid.UUID{s.pedicure, it.Prompt.ReturnServiceID}
	got, err := e.appts.Update(t.Context(), e.pool, it.Prompt.AppointmentID, appointments.AppointmentPatch{ServiceIDs: &ids})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	none(t, e.due(t, e.pool))
	if p := got.Services[0].PriceCents; p == nil || *p != 2500 {
		t.Fatalf("Pedicure line price = %v; want the saved 2500", p)
	}
	if want := later.EndsAt.Add(45 * time.Minute); !got.EndsAt.Equal(want) {
		t.Fatalf("ends_at = %v; want %v (grown by Relleno's 45 minutes)", got.EndsAt, want)
	}
}

func TestDueListArchivedClientNoItems(t *testing.T) {
	e := newDueEnv(t)
	s := newComeBack(t, e)
	e.rule(t, RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: e.template(t, "Cita", "Hola {{client.first_name}}"), IncludeOverdue: true})
	e.book(t, s.ana.ID, e.at(2026, 10, 23, 15, 0), s.pedicure)
	e.clock.Set(e.now())
	if items := e.due(t, e.pool); len(items) != 2 {
		t.Fatalf("before archiving: %d items; want the come-back and the before item", len(items))
	}

	if _, err := e.clients.Update(t.Context(), e.pool, s.ana.ID, clients.ClientPatch{Archived: ptr(true)}, "ES"); err != nil {
		t.Fatalf("archive client: %v", err)
	}
	none(t, e.due(t, e.pool))
}

func TestDueListRuleWithoutIncludeOverdueIsTodayOnward(t *testing.T) {
	e := newDueEnv(t)
	acrilico := e.service(t, "Acrílico", 60, nil)
	ana, marta := e.client(t, "Ana Rojas", false), e.client(t, "Marta Solano", false)
	e.clock.Set(e.at(2026, 8, 31, 10, 0))
	e.book(t, ana.ID, e.at(2026, 9, 1, 15, 0), acrilico) // due 2026-09-22: a month late
	e.book(t, marta.ID, e.at(2026, 10, 1, 15, 0), acrilico)
	e.clock.Set(e.now())
	e.rule(t, RuleInput{
		Kind: KindComeBack, ServiceID: nu(acrilico), OffsetDays: 21,
		TemplateID: e.template(t, "Vuelve", "Hola {{client.first_name}}"),
	})

	it := one(t, e.due(t, e.pool))
	if it.ClientID != marta.ID || it.Late {
		t.Fatalf("item = %+v; want only Marta's item due today, not Ana's late one", it)
	}
}

func TestDueListLoggedBeforeItemIsHidden(t *testing.T) {
	e := newDueEnv(t)
	ana := e.client(t, "Ana Rojas", true)
	r := e.rule(t, RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: e.template(t, "Cita", "Hola {{client.first_name}}"), IncludeOverdue: true})
	a := e.book(t, ana.ID, e.at(2026, 10, 23, 15, 0), e.service(t, "Pedicure", 30, nil))
	e.clock.Set(e.now())
	one(t, e.due(t, e.pool))

	e.record(t, LogEntry{RuleID: r.ID, AppointmentID: a.ID, AnchorAt: &a.StartsAt, Action: ActionSent, ActedAt: e.now()})
	none(t, e.due(t, e.pool))
}

// TestDueListUsesOneSnapshot reads the due list twice in one read-only REPEATABLE READ
// transaction, with a change committed in between. Both reads answer as the pool did before the
// change, and the pool sees the change: every load runs on q.
func TestDueListUsesOneSnapshot(t *testing.T) {
	e := newDueEnv(t)
	s := newComeBack(t, e)
	later := e.book(t, s.ana.ID, e.at(2026, 11, 12, 10, 0), s.relleno)
	e.clock.Set(e.now())
	before := e.due(t, e.pool)

	// Nothing inside the callback calls t.Fatal: Goexit would skip the rollback and hold the
	// connection, so the pool's cleanup would hang. The callback returns errors and the asserts
	// run after it.
	var first, second []DueItem
	err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		if first, err = e.m.DueList(t.Context(), tx, e.now()); err != nil {
			return err
		}
		if _, err = e.appts.SetStatus(t.Context(), e.pool, later.ID, appointments.Cancelled, false); err != nil {
			return err
		}
		second, err = e.m.DueList(t.Context(), tx, e.now())
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if !reflect.DeepEqual(first, before) || !reflect.DeepEqual(second, before) {
		t.Fatalf("snapshot reads = %+v, %+v; want both %+v", first, second, before)
	}
	one(t, e.due(t, e.pool))
}

func TestDueListBeforeSetupIsNotFound(t *testing.T) {
	_, err := New(Options{Clock: clock.NewFake(start), Attributes: attributes.New(attributes.Options{})}).DueList(t.Context(), dbtest.Pool(t), start)
	if !errors.As(err, &problem.NotFound{}) {
		t.Fatalf("err = %v; want problem.NotFound", err)
	}
}

func TestDueListUnreadableSettingsAreAnError(t *testing.T) {
	for _, column := range []string{"timezone", "locale"} {
		t.Run(column, func(t *testing.T) {
			e := newDueEnv(t)
			if _, err := e.pool.Exec(t.Context(), `UPDATE business_settings SET `+column+` = 'xx/unknown'`); err != nil {
				t.Fatalf("break settings: %v", err)
			}
			items, err := e.m.DueList(t.Context(), e.pool, e.now())
			if err == nil || items != nil {
				t.Fatalf("DueList = %v, %v; want an error and no items", items, err)
			}
		})
	}
}

// TestDueListDBErrors fails each load in turn, on data that reaches every query: a "before"
// candidate, a come-back candidate with a later booking, and a log row.
func TestDueListDBErrors(t *testing.T) {
	sweepDBErrors(t, func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		e := newDueEnv(t)
		s := newComeBack(t, e)
		r := e.rule(t, RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: e.template(t, "Cita", "Hola"), IncludeOverdue: true})
		e.book(t, s.ana.ID, e.at(2026, 11, 12, 10, 0), s.pedicure)
		soon := e.book(t, s.ana.ID, e.at(2026, 10, 23, 15, 0), s.pedicure)
		e.record(t, LogEntry{RuleID: r.ID, AppointmentID: soon.ID, AnchorAt: &soon.StartsAt, Action: ActionSkipped, ActedAt: e.now()})
		return e.pool, func(q db.DBTX) error { _, err := e.m.DueList(t.Context(), q, e.now()); return err }
	})
}
