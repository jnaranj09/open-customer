//go:build db

package clients

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var start = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

const region = "CR"

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
	return fixture{
		pool: dbtest.Pool(t), clock: fake, attrs: attrs,
		m: New(Options{Clock: fake, Attributes: attrs}),
	}
}

func ptr[T any](v T) *T { return &v }

func (f fixture) create(t *testing.T, in ClientInput) Client {
	t.Helper()
	c, err := f.m.Create(t.Context(), f.pool, in, region)
	if err != nil {
		t.Fatalf("Create %q: %v", in.Name, err)
	}
	return c
}

func (f fixture) addAttr(t *testing.T, label string, required bool) {
	t.Helper()
	_, err := f.attrs.Create(t.Context(), f.pool, attributes.AttributeInput{
		Label: label, Scope: attributes.ScopeClient, Type: attributes.TypeText, Required: required,
	})
	if err != nil {
		t.Fatalf("create attribute %q: %v", label, err)
	}
}

func fieldsOf(t *testing.T, err error) []problem.FieldError {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, want problem.Validation", err)
	}
	return v.Errors
}

// sameJSON compares two JSON objects by value: Postgres jsonb rewrites the spacing on read.
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

func TestCreateClientNormalizesPhone(t *testing.T) {
	f := newFixture(t)
	c := f.create(t, ClientInput{Name: "  Lucía Rojas ", Phone: ptr("8888-7777"), Notes: ptr("Prefiere tardes")})
	if c.Name != "Lucía Rojas" {
		t.Fatalf("name = %q, want it trimmed", c.Name)
	}
	if c.PhoneE164 == nil || *c.PhoneE164 != "+50688887777" || c.PhoneDigits == nil || *c.PhoneDigits != "50688887777" {
		t.Fatalf("phone = %v, %v, want +50688887777 and 50688887777", c.PhoneE164, c.PhoneDigits)
	}
	if string(c.Attrs) != "{}" {
		t.Fatalf("attrs = %s, want {}", c.Attrs)
	}
	if !c.CreatedAt.Equal(start) || !c.UpdatedAt.Equal(start) {
		t.Fatalf("created %v updated %v, want the clock time", c.CreatedAt, c.UpdatedAt)
	}
	got, err := f.m.Get(t.Context(), f.pool, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != c.Name || *got.PhoneE164 != *c.PhoneE164 || *got.Notes != "Prefiere tardes" {
		t.Fatalf("stored = %+v, want %+v", got, c)
	}
}

func TestCreateClientEmptyPhoneAndNotesAreNull(t *testing.T) {
	f := newFixture(t)
	c := f.create(t, ClientInput{Name: "Ana", Phone: ptr(""), Notes: ptr("  ")})
	if c.PhoneE164 != nil || c.PhoneDigits != nil || c.Notes != nil {
		t.Fatalf("phone %v %v notes %v, want all nil", c.PhoneE164, c.PhoneDigits, c.Notes)
	}
}

func TestCreateClientRejectsBadFields(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, ClientInput{
		Name: "  ", Phone: ptr("123"), Notes: ptr(string(make([]rune, 2001))),
	}, region)
	errs := fieldsOf(t, err)
	want := map[string]string{"name": problem.MsgClientNameInvalid, "phone": problem.MsgPhoneInvalid, "notes": problem.MsgNotesTooLong}
	if len(errs) != len(want) {
		t.Fatalf("errors = %+v, want %d", errs, len(want))
	}
	for _, e := range errs {
		if want[e.Field] != e.Message {
			t.Fatalf("error %+v, want message %q", e, want[e.Field])
		}
	}
}

func TestCreateRejectsRequiredAttr(t *testing.T) {
	f := newFixture(t)
	f.addAttr(t, "Alergias", true)
	_, err := f.m.Create(t.Context(), f.pool, ClientInput{Name: "Ana"}, region)
	errs := fieldsOf(t, err)
	if len(errs) != 1 || errs[0].Field != "attrs.alergias" {
		t.Fatalf("errors = %+v, want one on attrs.alergias", errs)
	}
	c := f.create(t, ClientInput{Name: "Ana", Attrs: json.RawMessage(`{"alergias":"ninguna"}`)})
	if string(c.Attrs) != `{"alergias":"ninguna"}` {
		t.Fatalf("attrs = %s", c.Attrs)
	}
}

func TestListArchivedOrActiveByName(t *testing.T) {
	f := newFixture(t)
	f.create(t, ClientInput{Name: "marta Solano"})
	f.create(t, ClientInput{Name: "Lucía Rojas"})
	v := f.create(t, ClientInput{Name: "Valeria Mora"})
	if _, err := f.m.Update(t.Context(), f.pool, v.ID, ClientPatch{Archived: ptr(true)}, region); err != nil {
		t.Fatal(err)
	}
	names := func(archived bool) []string {
		list, err := f.m.List(t.Context(), f.pool, archived)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(list))
		for _, c := range list {
			out = append(out, c.Name)
		}
		return out
	}
	if got := names(false); len(got) != 2 || got[0] != "Lucía Rojas" || got[1] != "marta Solano" {
		t.Fatalf("active = %v, want Lucía then marta", got)
	}
	if got := names(true); len(got) != 1 || got[0] != "Valeria Mora" {
		t.Fatalf("archived = %v, want Valeria only", got)
	}
}

func TestListEmptyIsNotNil(t *testing.T) {
	f := newFixture(t)
	list, err := f.m.List(t.Context(), f.pool, false)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("list = %v, %v, want an empty non-nil slice", list, err)
	}
}

func TestDuplicatePhoneWarns(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, ClientInput{Name: "Lucía", Phone: ptr("8888-7777")})
	_, err := f.m.Create(t.Context(), f.pool, ClientInput{Name: "Otra", Phone: ptr("+506 8888 7777")}, region)
	var dup DuplicatePhoneError
	if !errors.As(err, &dup) {
		t.Fatalf("err = %v, want DuplicatePhoneError", err)
	}
	if len(dup.Clients) != 1 || dup.Clients[0].ID != first.ID {
		t.Fatalf("clients = %+v, want the first client", dup.Clients)
	}
	var conflict problem.Conflict
	if !errors.As(err, &conflict) || conflict.Name != "duplicate-phone" {
		t.Fatalf("err = %v, want a problem.Conflict named duplicate-phone", err)
	}
	list, _ := f.m.List(t.Context(), f.pool, false)
	if len(list) != 1 {
		t.Fatalf("clients = %d, want the warned create to write nothing", len(list))
	}
}

func TestDuplicatePhoneAllowed(t *testing.T) {
	f := newFixture(t)
	f.create(t, ClientInput{Name: "Lucía", Phone: ptr("8888-7777")})
	f.create(t, ClientInput{Name: "Otra", Phone: ptr("8888-7777"), AllowDuplicatePhone: true})
	list, _ := f.m.List(t.Context(), f.pool, false)
	if len(list) != 2 {
		t.Fatalf("clients = %d, want 2", len(list))
	}
}

func TestDuplicatePhoneAcrossFormats(t *testing.T) {
	f := newFixture(t)
	first, err := f.m.Create(t.Context(), f.pool, ClientInput{Name: "Lucía", Phone: ptr("612 345 678")}, "ES")
	if err != nil {
		t.Fatal(err)
	}
	second := ClientInput{Name: "Otra", Phone: ptr("+34 612-345-678")}
	if _, err := f.m.Create(t.Context(), f.pool, second, "ES"); !errors.As(err, new(DuplicatePhoneError)) {
		t.Fatalf("err = %v, want DuplicatePhoneError across formats", err)
	}
	if _, err := f.m.Update(t.Context(), f.pool, first.ID, ClientPatch{Archived: ptr(true)}, "ES"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Create(t.Context(), f.pool, second, "ES"); err != nil {
		t.Fatalf("after archiving the first client: %v", err)
	}
}

func TestUpdateSamePhoneNoWarning(t *testing.T) {
	f := newFixture(t)
	f.create(t, ClientInput{Name: "Lucía", Phone: ptr("8888-7777")})
	b := f.create(t, ClientInput{Name: "Otra", Phone: ptr("8888-7777"), AllowDuplicatePhone: true})
	same := ClientPatch{Name: ptr("Otra Rojas"), Phone: nullable.NewNullableWithValue("+506 8888 7777")}
	got, err := f.m.Update(t.Context(), f.pool, b.ID, same, region)
	if err != nil {
		t.Fatalf("update with the stored phone: %v", err)
	}
	if got.Name != "Otra Rojas" {
		t.Fatalf("name = %q", got.Name)
	}
	c := f.create(t, ClientInput{Name: "Tercera", Phone: ptr("8999-0000")})
	_, err = f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Phone: nullable.NewNullableWithValue("8888-7777")}, region)
	if !errors.As(err, new(DuplicatePhoneError)) {
		t.Fatalf("err = %v, want DuplicatePhoneError when the phone changes to a taken one", err)
	}
	patch := ClientPatch{Phone: nullable.NewNullableWithValue("8888-7777"), AllowDuplicatePhone: true}
	if _, err := f.m.Update(t.Context(), f.pool, c.ID, patch, region); err != nil {
		t.Fatalf("allowed duplicate on update: %v", err)
	}
}

func TestUpdateClearsPhoneAndNotes(t *testing.T) {
	f := newFixture(t)
	c := f.create(t, ClientInput{Name: "Lucía", Phone: ptr("8888-7777"), Notes: ptr("hola")})
	f.clock.Advance(time.Hour)
	got, err := f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Phone: nullable.NewNullNullable[string](), Notes: nullable.NewNullableWithValue("")}, region)
	if err != nil {
		t.Fatal(err)
	}
	if got.PhoneE164 != nil || got.PhoneDigits != nil || got.Notes != nil {
		t.Fatalf("phone %v %v notes %v, want all cleared", got.PhoneE164, got.PhoneDigits, got.Notes)
	}
	if !got.UpdatedAt.Equal(start.Add(time.Hour)) || !got.CreatedAt.Equal(start) {
		t.Fatalf("created %v updated %v, want updated from the clock", got.CreatedAt, got.UpdatedAt)
	}
	again, err := f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Phone: nullable.NewNullableWithValue("")}, region)
	if err != nil || again.PhoneE164 != nil {
		t.Fatalf("empty phone string = %v, %v, want no phone", again.PhoneE164, err)
	}
}

func TestUpdateKeepsUnspecifiedFieldsAndArchives(t *testing.T) {
	f := newFixture(t)
	c := f.create(t, ClientInput{Name: "Lucía", Phone: ptr("8888-7777"), Notes: ptr("hola")})
	got, err := f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Archived: ptr(true)}, region)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Archived || got.Name != "Lucía" || *got.PhoneE164 != "+50688887777" || *got.Notes != "hola" {
		t.Fatalf("got %+v, want the other fields kept and archived", got)
	}
	got, err = f.m.Get(t.Context(), f.pool, c.ID)
	if err != nil || !got.Archived {
		t.Fatalf("Get archived = %+v, %v, want it returned archived", got, err)
	}
	got, err = f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Archived: ptr(false)}, region)
	if err != nil || got.Archived {
		t.Fatalf("restore = %+v, %v", got, err)
	}
}

func TestUpdateRejectsBadFields(t *testing.T) {
	f := newFixture(t)
	c := f.create(t, ClientInput{Name: "Lucía"})
	_, err := f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{
		Name: ptr(""), Phone: nullable.NewNullableWithValue("123"), Notes: nullable.NewNullableWithValue(string(make([]rune, 2001))),
	}, region)
	if errs := fieldsOf(t, err); len(errs) != 3 {
		t.Fatalf("errors = %+v, want name, phone and notes", errs)
	}
}

func TestUpdateMergesAttrs(t *testing.T) {
	f := newFixture(t)
	f.addAttr(t, "Alergias", false)
	f.addAttr(t, "Color favorito", false)
	c := f.create(t, ClientInput{Name: "Lucía", Attrs: json.RawMessage(`{"alergias":"látex","color_favorito":"rojo"}`)})
	got, err := f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Attrs: json.RawMessage(`{"color_favorito":"azul"}`)}, region)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, got.Attrs, `{"alergias":"látex","color_favorito":"azul"}`)
	if _, err = f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Name: ptr("Lucía R")}, region); err != nil {
		t.Fatal(err)
	}
	stored, err := f.m.Get(t.Context(), f.pool, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, stored.Attrs, `{"alergias":"látex","color_favorito":"azul"}`)
	_, err = f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Attrs: json.RawMessage(`{"nope":"x"}`)}, region)
	if errs := fieldsOf(t, err); len(errs) != 1 || errs[0].Field != "attrs.nope" {
		t.Fatalf("errors = %+v, want one on attrs.nope", errs)
	}
}

func TestGetUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	if _, err := f.m.Get(t.Context(), f.pool, uuid.New()); !errors.As(err, new(problem.NotFound)) {
		t.Fatalf("Get err = %v, want NotFound", err)
	}
	if _, err := f.m.Update(t.Context(), f.pool, uuid.New(), ClientPatch{Name: ptr("x")}, region); !errors.As(err, new(problem.NotFound)) {
		t.Fatalf("Update err = %v, want NotFound", err)
	}
}

func TestCreateClientDBErrorWritesNothing(t *testing.T) {
	f := newFixture(t)
	in := ClientInput{Name: "Lucía", Phone: ptr("8888-7777")}
	steps := 0
	if _, err := f.m.Create(t.Context(), dbtest.FailingDB{DBTX: f.pool, Calls: &steps}, in, region); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(t.Context(), `DELETE FROM clients`); err != nil {
		t.Fatal(err)
	}
	calls := 0
	// The insert is the last database call of Create.
	failing := dbtest.FailingDB{DBTX: f.pool, FailAt: steps, Calls: &calls}
	_, err := f.m.Create(t.Context(), failing, in, region)
	if !errors.Is(err, dbtest.ErrInjected) {
		t.Fatalf("err = %v, want the injected database error wrapped", err)
	}
	var n int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM clients`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("clients rows = %d, %v, want 0", n, err)
	}
}

// tickingClock moves one minute on every Now call, so two reads in one Update differ.
type tickingClock struct{ t time.Time }

func (c *tickingClock) Now() time.Time {
	c.t = c.t.Add(time.Minute)
	return c.t
}

func TestUpdateReadsTheClockOnce(t *testing.T) {
	ticking := &tickingClock{t: start}
	attrs := attributes.New(attributes.Options{Clock: ticking})
	m := New(Options{Clock: ticking, Attributes: attrs})
	pool := dbtest.Pool(t)
	c, err := m.Create(t.Context(), pool, ClientInput{Name: "Lucía"}, region)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Update(t.Context(), pool, c.ID, ClientPatch{Archived: ptr(true)}, region); err != nil {
		t.Fatal(err)
	}
	var archivedAt, updatedAt time.Time
	row := pool.QueryRow(t.Context(), `SELECT archived_at, updated_at FROM clients WHERE id = $1`, c.ID)
	if err := row.Scan(&archivedAt, &updatedAt); err != nil {
		t.Fatal(err)
	}
	if !archivedAt.Equal(updatedAt) {
		t.Fatalf("archived_at %v, updated_at %v, want the same clock reading", archivedAt, updatedAt)
	}
}

// archivedPair returns an archived client and an active one that share a phone.
func (f fixture) archivedPair(t *testing.T) (archived Client) {
	t.Helper()
	archived = f.create(t, ClientInput{Name: "Lucía", Phone: ptr("8888-7777")})
	if _, err := f.m.Update(t.Context(), f.pool, archived.ID, ClientPatch{Archived: ptr(true)}, region); err != nil {
		t.Fatal(err)
	}
	f.create(t, ClientInput{Name: "Otra", Phone: ptr("8888-7777")})
	return archived
}

func TestRestoreWithTakenPhoneWarns(t *testing.T) {
	f := newFixture(t)
	archived := f.archivedPair(t)
	_, err := f.m.Update(t.Context(), f.pool, archived.ID, ClientPatch{Archived: ptr(false)}, region)
	if !errors.As(err, new(DuplicatePhoneError)) {
		t.Fatalf("err = %v, want DuplicatePhoneError on restore", err)
	}
	got, err := f.m.Get(t.Context(), f.pool, archived.ID)
	if err != nil || !got.Archived {
		t.Fatalf("after the refused restore: %+v, %v, want still archived", got, err)
	}
}

func TestRestoreWithTakenPhoneAllowed(t *testing.T) {
	f := newFixture(t)
	archived := f.archivedPair(t)
	patch := ClientPatch{Archived: ptr(false), AllowDuplicatePhone: true}
	got, err := f.m.Update(t.Context(), f.pool, archived.ID, patch, region)
	if err != nil || got.Archived {
		t.Fatalf("restore with allow = %+v, %v, want active", got, err)
	}
}

func TestRestoreWithoutCollisionSucceeds(t *testing.T) {
	f := newFixture(t)
	c := f.create(t, ClientInput{Name: "Lucía", Phone: ptr("8888-7777")})
	if _, err := f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Archived: ptr(true)}, region); err != nil {
		t.Fatal(err)
	}
	got, err := f.m.Update(t.Context(), f.pool, c.ID, ClientPatch{Archived: ptr(false)}, region)
	if err != nil || got.Archived {
		t.Fatalf("restore = %+v, %v, want active", got, err)
	}
}

func TestPatchArchivedFalseOnActiveClientSkipsDuplicateCheck(t *testing.T) {
	f := newFixture(t)
	f.create(t, ClientInput{Name: "Lucía", Phone: ptr("8888-7777")})
	b := f.create(t, ClientInput{Name: "Otra", Phone: ptr("8888-7777"), AllowDuplicatePhone: true})
	if _, err := f.m.Update(t.Context(), f.pool, b.ID, ClientPatch{Archived: ptr(false)}, region); err != nil {
		t.Fatalf("archived:false on an active client: %v", err)
	}
}
