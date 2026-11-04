//go:build db

package appointments

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/services"
)

// now is the fake clock's start; at is a booking time two days later.
var (
	now = time.Date(2030, 3, 2, 9, 0, 0, 0, time.UTC)
	at  = time.Date(2030, 3, 4, 14, 0, 0, 0, time.UTC)
)

type fixture struct {
	pool     *pgxpool.Pool
	clock    *clock.Fake
	services *services.Module
	attrs    *attributes.Module
	m        *Module
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	fake := clock.NewFake(now)
	svcs := services.New(services.Options{Clock: fake})
	attrs := attributes.New(attributes.Options{Clock: fake})
	return fixture{
		pool: dbtest.Pool(t), clock: fake, services: svcs, attrs: attrs,
		m: New(Options{Clock: fake, Services: svcs, Attributes: attrs}),
	}
}

func str(s string) *string                 { return &s }
func tm(t time.Time) *time.Time            { return &t }
func svcIDs(list ...uuid.UUID) []uuid.UUID { return list }

// client stores a made-up client directly: the clients module is not this module's dependency.
func (f fixture) client(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := f.pool.Exec(t.Context(), `INSERT INTO clients (id, name) VALUES ($1, $2)`, id, name); err != nil {
		t.Fatalf("insert client: %v", err)
	}
	return id
}

func (f fixture) archiveClient(t *testing.T, id uuid.UUID) {
	t.Helper()
	if _, err := f.pool.Exec(t.Context(), `UPDATE clients SET archived_at = now() WHERE id = $1`, id); err != nil {
		t.Fatalf("archive client: %v", err)
	}
}

func (f fixture) service(t *testing.T, name string, dur int, price *int64) services.Service {
	t.Helper()
	s, err := f.services.Create(t.Context(), f.pool, services.ServiceInput{
		Name: name, DurationMin: dur, PriceCents: price, Color: "#aa3366",
	})
	if err != nil {
		t.Fatalf("create service %q: %v", name, err)
	}
	return s
}

func (f fixture) editService(t *testing.T, id uuid.UUID, p services.ServicePatch) {
	t.Helper()
	if _, err := f.services.Update(t.Context(), f.pool, id, p); err != nil {
		t.Fatalf("update service: %v", err)
	}
}

func (f fixture) archiveService(t *testing.T, id uuid.UUID) {
	t.Helper()
	archived := true
	f.editService(t, id, services.ServicePatch{Archived: &archived})
}

func (f fixture) attribute(t *testing.T, label string, required bool) {
	t.Helper()
	if _, err := f.attrs.Create(t.Context(), f.pool, attributes.AttributeInput{
		Label: label, Scope: attributes.ScopeAppointment, Type: attributes.TypeText, Required: required,
	}); err != nil {
		t.Fatalf("create attribute %q: %v", label, err)
	}
}

func (f fixture) create(t *testing.T, in AppointmentInput) Appointment {
	t.Helper()
	a, err := f.m.Create(t.Context(), f.pool, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return a
}

// book stores an appointment from start to end with no service lines.
func (f fixture) book(t *testing.T, clientID uuid.UUID, start, end time.Time) Appointment {
	t.Helper()
	return f.create(t, AppointmentInput{ClientID: clientID, StartsAt: start, EndsAt: &end})
}

func (f fixture) update(t *testing.T, id uuid.UUID, p AppointmentPatch) Appointment {
	t.Helper()
	a, err := f.m.Update(t.Context(), f.pool, id, p)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	return a
}

func (f fixture) get(t *testing.T, id uuid.UUID) Appointment {
	t.Helper()
	a, err := f.m.Get(t.Context(), f.pool, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return a
}

func (f fixture) setStatus(t *testing.T, id uuid.UUID, to Status) Appointment {
	t.Helper()
	a, err := f.m.SetStatus(t.Context(), f.pool, id, to, false)
	if err != nil {
		t.Fatalf("SetStatus %s: %v", to, err)
	}
	return a
}

func (f fixture) count(t *testing.T, table string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func wantFields(t *testing.T, err error, want ...problem.FieldError) {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, want problem.Validation %v", err, want)
	}
	if !reflect.DeepEqual(v.Errors, want) {
		t.Fatalf("field errors = %v, want %v", v.Errors, want)
	}
}

func fe(field, msg string) problem.FieldError { return problem.FieldError{Field: field, Message: msg} }

func wantLines(t *testing.T, got []Line, want ...Line) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("lines = %+v, want %+v", got, want)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.ServiceID != w.ServiceID || g.Name != w.Name || g.DurationMin != w.DurationMin ||
			!reflect.DeepEqual(g.PriceCents, w.PriceCents) {
			t.Fatalf("line %d = %+v, want %+v", i, g, w)
		}
	}
}

func lineOf(s services.Service) Line {
	return Line{ServiceID: s.ID, Name: s.Name, PriceCents: s.PriceCents, DurationMin: s.DurationMin}
}

func sameJSON(t *testing.T, got json.RawMessage, want string) {
	t.Helper()
	var g, w map[string]any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("attrs %s: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("attrs = %s, want %s", got, want)
	}
}

func TestCreateCopiesServicesAndSumsTotals(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Lucía Rojas")
	a := f.service(t, "Corte", 60, ptr(2500))
	b := f.service(t, "Lavado", 30, nil)
	got := f.create(t, AppointmentInput{ClientID: c, StartsAt: at, ServiceIDs: svcIDs(a.ID, b.ID)})
	if !got.EndsAt.Equal(at.Add(90*time.Minute)) || got.TotalCents != 2500 || got.PaidCents != nil {
		t.Fatalf("ends %v total %d paid %v, want start+90m, 2500, nil", got.EndsAt, got.TotalCents, got.PaidCents)
	}
	if got.Status != Scheduled || got.Done || got.ClientID != c || got.ClientName != "Lucía Rojas" {
		t.Fatalf("got %+v, want a scheduled, not done appointment of Lucía Rojas", got)
	}
	if !got.CreatedAt.Equal(now) || !got.UpdatedAt.Equal(now) || string(got.Attrs) != "{}" || got.Notes != nil {
		t.Fatalf("created %v updated %v attrs %s notes %v", got.CreatedAt, got.UpdatedAt, got.Attrs, got.Notes)
	}
	wantLines(t, got.Services, lineOf(a), lineOf(b))
	stored := f.get(t, got.ID)
	if !stored.StartsAt.Equal(at) || !stored.EndsAt.Equal(got.EndsAt) || stored.TotalCents != 2500 {
		t.Fatalf("stored = %+v, want the created appointment", stored)
	}
	wantLines(t, stored.Services, lineOf(a), lineOf(b))
}

func TestCreateSameServiceTwice(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	got := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(a.ID, a.ID)})
	if !got.EndsAt.Equal(at.Add(2*time.Hour)) || got.TotalCents != 5000 {
		t.Fatalf("ends %v total %d, want start+2h and 5000", got.EndsAt, got.TotalCents)
	}
	wantLines(t, f.get(t, got.ID).Services, lineOf(a), lineOf(a))
}

func TestCreateExplicitEndsAtKept(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	end := at.Add(45 * time.Minute)
	got := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, EndsAt: &end, ServiceIDs: svcIDs(a.ID)})
	if !got.EndsAt.Equal(end) || got.TotalCents != 2500 {
		t.Fatalf("ends %v total %d, want the given end and 2500", got.EndsAt, got.TotalCents)
	}
}

func TestCreateWithoutServicesHasEmptyLines(t *testing.T) {
	f := newFixture(t)
	got := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
	if got.Services == nil || len(got.Services) != 0 || got.TotalCents != 0 {
		t.Fatalf("services %v total %d, want an empty non-nil list and 0", got.Services, got.TotalCents)
	}
}

func TestCreateNoServicesNoEndIs422(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs()})
	wantFields(t, err, fe("service_ids", problem.MsgServicesOrEndRequired))
}

func TestCreateEndBeforeStartIs422(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	for name, end := range map[string]time.Time{"equal": at, "before": at.Add(-time.Minute)} {
		t.Run(name, func(t *testing.T) {
			_, err := f.m.Create(t.Context(), f.pool, AppointmentInput{ClientID: c, StartsAt: at, EndsAt: &end})
			wantFields(t, err, fe("ends_at", problem.MsgEndBeforeStart))
		})
	}
	if n := f.count(t, "appointments"); n != 0 {
		t.Fatalf("appointments = %d, want 0", n)
	}
}

func TestCreateArchivedClientIs422(t *testing.T) {
	f := newFixture(t)
	archived := f.client(t, "Valeria Mora")
	f.archiveClient(t, archived)
	for name, c := range map[string]uuid.UUID{"archived": archived, "unknown": uuid.New()} {
		t.Run(name, func(t *testing.T) {
			_, err := f.m.Create(t.Context(), f.pool, AppointmentInput{ClientID: c, StartsAt: at, EndsAt: tm(at.Add(time.Hour))})
			wantFields(t, err, fe("client_id", problem.MsgClientUnavailable))
		})
	}
}

func TestCreateArchivedServiceIs422(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	f.archiveService(t, a.ID)
	_, err := f.m.Create(t.Context(), f.pool, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(a.ID)})
	wantFields(t, err, fe("service_ids", problem.MsgServiceUnavailable))
}

func TestCreateReportsEveryBadField(t *testing.T) {
	f := newFixture(t)
	archived := f.client(t, "Valeria Mora")
	f.archiveClient(t, archived)
	_, err := f.m.Create(t.Context(), f.pool, AppointmentInput{
		ClientID: archived, StartsAt: at, EndsAt: tm(at), Notes: str(strings.Repeat("a", 2001)),
		Attrs: json.RawMessage(`{"nada":"x"}`),
	})
	wantFields(t, err,
		fe("client_id", problem.MsgClientUnavailable),
		fe("ends_at", problem.MsgEndBeforeStart),
		fe("attrs.nada", "El dato 'nada' no existe"),
		fe("notes", problem.MsgNotesTooLong),
	)
}

func TestCreateBadServiceSkipsTheEndCheck(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(uuid.New())})
	wantFields(t, err, fe("service_ids", problem.MsgServiceUnavailable))
}

func TestCreateNotesTrimmedAndBlankIsNull(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	end := at.Add(time.Hour)
	got := f.create(t, AppointmentInput{ClientID: c, StartsAt: at, EndsAt: &end, Notes: str("  Trae su tinte ")})
	if got.Notes == nil || *got.Notes != "Trae su tinte" {
		t.Fatalf("notes = %v, want trimmed", got.Notes)
	}
	blank := f.create(t, AppointmentInput{ClientID: c, StartsAt: at.Add(2 * time.Hour), EndsAt: tm(at.Add(3 * time.Hour)), Notes: str("   ")})
	if blank.Notes != nil {
		t.Fatalf("notes = %q, want NULL", *blank.Notes)
	}
	full := f.create(t, AppointmentInput{ClientID: c, StartsAt: at.Add(4 * time.Hour), EndsAt: tm(at.Add(5 * time.Hour)), Notes: str(strings.Repeat("ñ", 2000))})
	if full.Notes == nil || len([]rune(*full.Notes)) != 2000 {
		t.Fatal("2000 runes of notes must be accepted")
	}
}

func TestCreateValidatesAppointmentAttrs(t *testing.T) {
	f := newFixture(t)
	f.attribute(t, "Bebida", false)
	c := f.client(t, "Marta Solano")
	got := f.create(t, AppointmentInput{ClientID: c, StartsAt: at, EndsAt: tm(at.Add(time.Hour)), Attrs: json.RawMessage(`{"bebida":" café "}`)})
	sameJSON(t, f.get(t, got.ID).Attrs, `{"bebida":"café"}`)
	f.attribute(t, "Motivo", true)
	_, err := f.m.Create(t.Context(), f.pool, AppointmentInput{ClientID: c, StartsAt: at, EndsAt: tm(at.Add(time.Hour)), AllowOverlap: true})
	wantFields(t, err, fe("attrs.motivo", "Motivo es obligatorio"))
}

func TestCreateLineInsertFailureLeavesNoAppointment(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	c := f.client(t, "Marta Solano")
	if _, err := f.pool.Exec(t.Context(), `CREATE FUNCTION fail_line() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'made-up'; END$$`); err != nil {
		t.Fatalf("create function: %v", err)
	}
	if _, err := f.pool.Exec(t.Context(), `CREATE TRIGGER fail_line BEFORE INSERT ON appointment_services FOR EACH ROW EXECUTE FUNCTION fail_line()`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	err := db.WithTx(t.Context(), f.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := f.m.Create(t.Context(), tx, AppointmentInput{ClientID: c, StartsAt: at, ServiceIDs: svcIDs(a.ID)})
		return err
	})
	// The error must be the trigger's, from the line insert: not a later statement failing on
	// the aborted transaction.
	if err == nil || !strings.Contains(err.Error(), "insert appointment service") || !strings.Contains(err.Error(), "made-up") {
		t.Fatalf("err = %v, want the line insert error raised by the trigger", err)
	}
	if n := f.count(t, "appointments"); n != 0 {
		t.Fatalf("appointments = %d, want 0 after the rollback", n)
	}
}

func TestPriceSnapshotSurvivesServiceEdit(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(a.ID)})
	dur := 90
	f.editService(t, a.ID, services.ServicePatch{Name: str("Corte largo"), DurationMin: &dur, PriceCents: nullable.NewNullableWithValue[int64](3000)})
	got := f.get(t, booked.ID)
	wantLines(t, got.Services, lineOf(a))
	if got.TotalCents != 2500 || !got.EndsAt.Equal(at.Add(time.Hour)) {
		t.Fatalf("total %d ends %v, want the booked 2500 and start+1h", got.TotalCents, got.EndsAt)
	}
}

// C3: services may not read appointments, so this services rule is tested here.
func TestArchivedServiceStaysReadableOnPastAppointment(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	c := f.client(t, "Marta Solano")
	booked := f.create(t, AppointmentInput{ClientID: c, StartsAt: at, ServiceIDs: svcIDs(a.ID)})
	f.archiveService(t, a.ID)
	f.clock.Set(at.Add(24 * time.Hour))
	got := f.get(t, booked.ID)
	wantLines(t, got.Services, lineOf(a))
	if !got.Done || got.TotalCents != 2500 {
		t.Fatalf("done %v total %d, want a done past visit with its total", got.Done, got.TotalCents)
	}
	list, err := f.m.ListForClient(t.Context(), f.pool, c)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListForClient = %v, %v", list, err)
	}
	wantLines(t, list[0].Services, lineOf(a))
}

func TestUpdateServiceIDsRecopiesAndKeepsPaid(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	b := f.service(t, "Tinte", 30, ptr(4000))
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(a.ID)})
	f.update(t, booked.ID, AppointmentPatch{PaidCents: nullable.NewNullableWithValue[int64](1000)})
	got := f.update(t, booked.ID, AppointmentPatch{ServiceIDs: &[]uuid.UUID{b.ID}})
	wantLines(t, got.Services, lineOf(b))
	if got.TotalCents != 4000 || got.PaidCents == nil || *got.PaidCents != 1000 {
		t.Fatalf("total %d paid %v, want 4000 and the kept 1000", got.TotalCents, got.PaidCents)
	}
	wantLines(t, f.get(t, booked.ID).Services, lineOf(b))
}

// D10 and ruling F12: an active service is copied again at today's price.
func TestUpdateServiceIDsRecopiesActiveAtCurrentPrice(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(a.ID)})
	f.editService(t, a.ID, services.ServicePatch{PriceCents: nullable.NewNullableWithValue[int64](3000)})
	got := f.update(t, booked.ID, AppointmentPatch{ServiceIDs: &[]uuid.UUID{a.ID}})
	a.PriceCents = ptr(3000)
	wantLines(t, got.Services, lineOf(a))
	if got.TotalCents != 3000 {
		t.Fatalf("total = %d, want 3000", got.TotalCents)
	}
}

func TestUpdateEndsAtRecomputedWithoutEndsAt(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	b := f.service(t, "Lavado", 30, nil)
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, EndsAt: tm(at.Add(45 * time.Minute)), ServiceIDs: svcIDs(a.ID)})
	got := f.update(t, booked.ID, AppointmentPatch{ServiceIDs: &[]uuid.UUID{a.ID, b.ID}})
	if !got.EndsAt.Equal(at.Add(90 * time.Minute)) {
		t.Fatalf("ends = %v, want start+90m", got.EndsAt)
	}
}

func TestUpdateEndsAtKeptWhenSent(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	b := f.service(t, "Lavado", 30, nil)
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(a.ID)})
	end := at.Add(50 * time.Minute)
	got := f.update(t, booked.ID, AppointmentPatch{ServiceIDs: &[]uuid.UUID{a.ID, b.ID}, EndsAt: &end})
	if !got.EndsAt.Equal(end) {
		t.Fatalf("ends = %v, want the sent %v", got.EndsAt, end)
	}
}

func TestUpdateEmptyServiceIDs(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(a.ID)})
	_, err := f.m.Update(t.Context(), f.pool, booked.ID, AppointmentPatch{ServiceIDs: &[]uuid.UUID{}})
	wantFields(t, err, fe("service_ids", problem.MsgServicesOrEndRequired))
	got := f.update(t, booked.ID, AppointmentPatch{ServiceIDs: &[]uuid.UUID{}, EndsAt: tm(at.Add(time.Hour))})
	if len(got.Services) != 0 || got.Services == nil || got.TotalCents != 0 {
		t.Fatalf("services %v total %d, want none and 0", got.Services, got.TotalCents)
	}
}

func TestUpdateMovingStartMovesEnd(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, EndsAt: tm(at.Add(45 * time.Minute)), ServiceIDs: svcIDs(a.ID)})
	got := f.update(t, booked.ID, AppointmentPatch{StartsAt: tm(at.Add(2 * time.Hour))})
	if !got.StartsAt.Equal(at.Add(2*time.Hour)) || !got.EndsAt.Equal(at.Add(2*time.Hour+45*time.Minute)) {
		t.Fatalf("got %v–%v, want both moved by 2h", got.StartsAt, got.EndsAt)
	}
	wantLines(t, got.Services, lineOf(a))
}

func TestUpdateEndBeforeStartIs422(t *testing.T) {
	f := newFixture(t)
	booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
	_, err := f.m.Update(t.Context(), f.pool, booked.ID, AppointmentPatch{EndsAt: tm(at)})
	wantFields(t, err, fe("ends_at", problem.MsgEndBeforeStart))
	_, err = f.m.Update(t.Context(), f.pool, booked.ID, AppointmentPatch{StartsAt: tm(at.Add(2 * time.Hour)), EndsAt: tm(at.Add(time.Hour))})
	wantFields(t, err, fe("ends_at", problem.MsgEndBeforeStart))
}

func TestUpdatePastDoneAppointment(t *testing.T) {
	f := newFixture(t)
	booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
	f.clock.Set(at.Add(3 * time.Hour))
	if !f.get(t, booked.ID).Done {
		t.Fatal("the past appointment must be done")
	}
	got := f.update(t, booked.ID, AppointmentPatch{Notes: nullable.NewNullableWithValue("Llegó tarde"), EndsAt: tm(at.Add(2 * time.Hour))})
	if !got.Done || got.Notes == nil || *got.Notes != "Llegó tarde" || !got.UpdatedAt.Equal(at.Add(3*time.Hour)) {
		t.Fatalf("got %+v, want the edit saved and done still true", got)
	}
}

func TestUpdateKeepsLinesOfServicesArchivedLater(t *testing.T) {
	f := newFixture(t)
	a := f.service(t, "Corte", 60, ptr(2500))
	b := f.service(t, "Lavado", 30, ptr(800))
	c := f.service(t, "Peinado", 30, ptr(900))
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(a.ID)})
	f.archiveService(t, a.ID)
	f.archiveService(t, c.ID)
	got := f.update(t, booked.ID, AppointmentPatch{ServiceIDs: &[]uuid.UUID{a.ID, b.ID}})
	wantLines(t, got.Services, lineOf(a), lineOf(b))
	if got.TotalCents != 3300 || !got.EndsAt.Equal(at.Add(90*time.Minute)) {
		t.Fatalf("total %d ends %v, want 3300 and start+90m", got.TotalCents, got.EndsAt)
	}
	_, err := f.m.Update(t.Context(), f.pool, booked.ID, AppointmentPatch{ServiceIDs: &[]uuid.UUID{a.ID, b.ID, c.ID}})
	wantFields(t, err, fe("service_ids", problem.MsgServiceUnavailable))
}

func TestUpdateAttrsMerge(t *testing.T) {
	f := newFixture(t)
	f.attribute(t, "Bebida", false)
	f.attribute(t, "Música", false)
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, EndsAt: tm(at.Add(time.Hour)), Attrs: json.RawMessage(`{"bebida":"café"}`)})
	got := f.update(t, booked.ID, AppointmentPatch{Attrs: json.RawMessage(`{"musica":"jazz"}`)})
	sameJSON(t, got.Attrs, `{"bebida":"café","musica":"jazz"}`)
	got = f.update(t, booked.ID, AppointmentPatch{Attrs: json.RawMessage(`{"bebida":null}`)})
	sameJSON(t, got.Attrs, `{"musica":"jazz"}`)
	got = f.update(t, booked.ID, AppointmentPatch{Notes: nullable.NewNullableWithValue("x")})
	sameJSON(t, got.Attrs, `{"musica":"jazz"}`)
}

func TestUpdatePaidCents(t *testing.T) {
	f := newFixture(t)
	booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
	got := f.update(t, booked.ID, AppointmentPatch{PaidCents: nullable.NewNullableWithValue[int64](1500)})
	if got.PaidCents == nil || *got.PaidCents != 1500 {
		t.Fatalf("paid = %v, want 1500", got.PaidCents)
	}
	if got = f.update(t, booked.ID, AppointmentPatch{Notes: nullable.NewNullableWithValue("x")}); got.PaidCents == nil || *got.PaidCents != 1500 {
		t.Fatalf("paid = %v, want 1500 kept when absent", got.PaidCents)
	}
	if got = f.update(t, booked.ID, AppointmentPatch{PaidCents: nullable.NewNullNullable[int64]()}); got.PaidCents != nil {
		t.Fatalf("paid = %d, want cleared", *got.PaidCents)
	}
	_, err := f.m.Update(t.Context(), f.pool, booked.ID, AppointmentPatch{PaidCents: nullable.NewNullableWithValue[int64](-1)})
	wantFields(t, err, fe("paid_cents", problem.MsgPaidInvalid))
	if got = f.update(t, booked.ID, AppointmentPatch{PaidCents: nullable.NewNullableWithValue[int64](0)}); got.PaidCents == nil || *got.PaidCents != 0 {
		t.Fatalf("paid = %v, want 0 accepted", got.PaidCents)
	}
}

func TestUpdateNotes(t *testing.T) {
	f := newFixture(t)
	booked := f.create(t, AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, EndsAt: tm(at.Add(time.Hour)), Notes: str("Primera vez")})
	if got := f.update(t, booked.ID, AppointmentPatch{PaidCents: nullable.NewNullableWithValue[int64](1)}); got.Notes == nil || *got.Notes != "Primera vez" {
		t.Fatalf("notes = %v, want kept when absent", got.Notes)
	}
	if got := f.update(t, booked.ID, AppointmentPatch{Notes: nullable.NewNullableWithValue("  ")}); got.Notes != nil {
		t.Fatalf("notes = %q, want blank cleared", *got.Notes)
	}
	f.update(t, booked.ID, AppointmentPatch{Notes: nullable.NewNullableWithValue("Otra")})
	if got := f.update(t, booked.ID, AppointmentPatch{Notes: nullable.NewNullNullable[string]()}); got.Notes != nil {
		t.Fatalf("notes = %q, want null cleared", *got.Notes)
	}
	_, err := f.m.Update(t.Context(), f.pool, booked.ID, AppointmentPatch{Notes: nullable.NewNullableWithValue(strings.Repeat("a", 2001))})
	wantFields(t, err, fe("notes", problem.MsgNotesTooLong))
}

func TestUpdateReportsEveryBadField(t *testing.T) {
	f := newFixture(t)
	booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
	_, err := f.m.Update(t.Context(), f.pool, booked.ID, AppointmentPatch{
		ServiceIDs: &[]uuid.UUID{uuid.New()}, EndsAt: tm(at), Attrs: json.RawMessage(`{"nada":1}`),
		PaidCents: nullable.NewNullableWithValue[int64](-5), Notes: nullable.NewNullableWithValue(strings.Repeat("a", 2001)),
	})
	wantFields(t, err,
		fe("service_ids", problem.MsgServiceUnavailable),
		fe("ends_at", problem.MsgEndBeforeStart),
		fe("attrs.nada", "El dato 'nada' no existe"),
		fe("paid_cents", problem.MsgPaidInvalid),
		fe("notes", problem.MsgNotesTooLong),
	)
	got := f.get(t, booked.ID)
	if !got.EndsAt.Equal(at.Add(time.Hour)) || got.PaidCents != nil || got.Notes != nil {
		t.Fatalf("stored = %+v, want nothing changed", got)
	}
}

func TestUnknownAppointmentIsNotFound(t *testing.T) {
	f := newFixture(t)
	id := uuid.New()
	_, getErr := f.m.Get(t.Context(), f.pool, id)
	_, updErr := f.m.Update(t.Context(), f.pool, id, AppointmentPatch{Notes: nullable.NewNullableWithValue("x")})
	_, stErr := f.m.SetStatus(t.Context(), f.pool, id, Cancelled, false)
	for name, err := range map[string]error{"Get": getErr, "Update": updErr, "SetStatus": stErr} {
		if !errors.As(err, new(problem.NotFound)) {
			t.Errorf("%s err = %v, want problem.NotFound", name, err)
		}
	}
}

func TestSetStatusStoresTheChange(t *testing.T) {
	f := newFixture(t)
	booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
	f.clock.Advance(time.Minute)
	got := f.setStatus(t, booked.ID, Confirmed)
	stored := f.get(t, booked.ID)
	if got.Status != Confirmed || stored.Status != Confirmed || !stored.UpdatedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("got %s, stored %s at %v; want confirmed at the clock time", got.Status, stored.Status, stored.UpdatedAt)
	}
}

func TestSetStatusInvalidTransitionIs409(t *testing.T) {
	f := newFixture(t)
	booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
	f.setStatus(t, booked.ID, Cancelled)
	_, err := f.m.SetStatus(t.Context(), f.pool, booked.ID, Confirmed, false)
	var c problem.Conflict
	if !errors.As(err, &c) || c.Name != "invalid-transition" {
		t.Fatalf("err = %v, want Conflict invalid-transition", err)
	}
	if got := f.get(t, booked.ID); got.Status != Cancelled {
		t.Fatalf("status = %s, want cancelled kept", got.Status)
	}
}

func TestNoShowAfterDone(t *testing.T) {
	f := newFixture(t)
	booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
	f.clock.Set(at.Add(26 * time.Hour))
	if !f.get(t, booked.ID).Done {
		t.Fatal("the past appointment must be done")
	}
	got := f.setStatus(t, booked.ID, NoShow)
	if got.Status != NoShow || got.Done {
		t.Fatalf("status %s done %v, want no_show and not done", got.Status, got.Done)
	}
}

type dbOperation struct {
	name  string
	build func(t *testing.T) (db.DBTX, func(q db.DBTX) error)
}

var dbOperations = []dbOperation{
	{"Create", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		in := AppointmentInput{ClientID: f.client(t, "Marta Solano"), StartsAt: at, ServiceIDs: svcIDs(f.service(t, "Corte", 60, ptr(2500)).ID)}
		return f.pool, func(q db.DBTX) error { _, err := f.m.Create(t.Context(), q, in); return err }
	}},
	{"Update", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
		p := AppointmentPatch{
			StartsAt: tm(at.Add(time.Hour)), ServiceIDs: &[]uuid.UUID{f.service(t, "Corte", 60, ptr(2500)).ID},
			Attrs: json.RawMessage(`{}`), Notes: nullable.NewNullableWithValue("x"),
		}
		return f.pool, func(q db.DBTX) error { _, err := f.m.Update(t.Context(), q, booked.ID, p); return err }
	}},
	{"SetStatus", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
		f.setStatus(t, booked.ID, Cancelled)
		return f.pool, func(q db.DBTX) error {
			_, err := f.m.SetStatus(t.Context(), q, booked.ID, Scheduled, false)
			return err
		}
	}},
	{"ListRange", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
		return f.pool, func(q db.DBTX) error { _, err := f.m.ListRange(t.Context(), q, at, at, time.UTC); return err }
	}},
	{"ListForClient", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		c := f.client(t, "Marta Solano")
		f.book(t, c, at, at.Add(time.Hour))
		return f.pool, func(q db.DBTX) error { _, err := f.m.ListForClient(t.Context(), q, c); return err }
	}},
}

// TestAppointmentsDBErrors fails each database call of each operation in turn. The caller must
// get the database error back wrapped: never a success, and never a problem that hides an outage.
func TestAppointmentsDBErrors(t *testing.T) {
	for _, op := range dbOperations {
		t.Run(op.name, func(t *testing.T) {
			steps := 0
			pool, run := op.build(t)
			if err := run(dbtest.FailingDB{DBTX: pool, Calls: &steps}); err != nil {
				t.Fatalf("clean run: %v", err)
			}
			if steps == 0 {
				t.Fatal("clean run made no database call")
			}
			for step := 1; step <= steps; step++ {
				calls := 0
				pool, run := op.build(t)
				err := run(dbtest.FailingDB{DBTX: pool, FailAt: step, Calls: &calls})
				if !errors.Is(err, dbtest.ErrInjected) {
					t.Fatalf("failure at step %d of %d: err = %v; want the database error wrapped", step, steps, err)
				}
			}
		})
	}
}
