//go:build db

package services

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var start = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

type fixture struct {
	pool  *pgxpool.Pool
	clock *clock.Fake
	m     *Module
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	fake := clock.NewFake(start)
	return fixture{pool: dbtest.Pool(t), clock: fake, m: New(Options{Clock: fake})}
}

func ptr[T any](v T) *T { return &v }

func validInput() ServiceInput {
	return ServiceInput{Name: "Acrílico", DurationMin: 90, PriceCents: ptr(int64(25000)), Color: "#aa3366"}
}

func (f fixture) create(t *testing.T, in ServiceInput) Service {
	t.Helper()
	s, err := f.m.Create(t.Context(), f.pool, in)
	if err != nil {
		t.Fatalf("Create %q: %v", in.Name, err)
	}
	return s
}

func fieldsOf(t *testing.T, err error) []problem.FieldError {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, want problem.Validation", err)
	}
	return v.Errors
}

func fieldNames(errs []problem.FieldError) []string {
	names := make([]string, 0, len(errs))
	for _, e := range errs {
		names = append(names, e.Field)
	}
	return names
}

func TestCreateServiceRejectsBadFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ServiceInput)
		field  string
		msg    string
	}{
		{"duration 0", func(in *ServiceInput) { in.DurationMin = 0 }, "duration_min", problem.MsgDurationInvalid},
		{"duration 1441", func(in *ServiceInput) { in.DurationMin = 1441 }, "duration_min", problem.MsgDurationInvalid},
		{"price -1", func(in *ServiceInput) { in.PriceCents = ptr(int64(-1)) }, "price_cents", problem.MsgPriceInvalid},
		{"color #xyz", func(in *ServiceInput) { in.Color = "#xyz" }, "color", problem.MsgColorInvalid},
		{"empty name", func(in *ServiceInput) { in.Name = "" }, "name", problem.MsgNameInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			in := validInput()
			tc.mutate(&in)
			_, err := f.m.Create(t.Context(), f.pool, in)
			got := fieldsOf(t, err)
			if len(got) != 1 || got[0].Field != tc.field || got[0].Message != tc.msg {
				t.Fatalf("errors = %+v, want one on %q with %q", got, tc.field, tc.msg)
			}
		})
	}
}

func TestCreateServiceAcceptsBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   ServiceInput
	}{
		{"duration 1, price 0", ServiceInput{Name: "A", DurationMin: 1, PriceCents: ptr(int64(0)), Color: "#000000"}},
		{"duration 1440, no price", ServiceInput{Name: "B", DurationMin: 1440, Color: "#ffffff"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			s := f.create(t, tc.in)
			if s.DurationMin != tc.in.DurationMin {
				t.Fatalf("duration = %d, want %d", s.DurationMin, tc.in.DurationMin)
			}
		})
	}
}

func TestCreateServiceCollectsEveryError(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, ServiceInput{Name: "", DurationMin: 0, PriceCents: ptr(int64(-5)), Color: "red"})
	got := fieldNames(fieldsOf(t, err))
	want := []string{"name", "duration_min", "price_cents", "color"}
	if len(got) != len(want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fields = %v, want %v", got, want)
		}
	}
}

func TestCreateServiceNormalizesColor(t *testing.T) {
	f := newFixture(t)
	in := validInput()
	in.Color = "#AA3366"
	s := f.create(t, in)
	var stored string
	if err := f.pool.QueryRow(t.Context(), `SELECT color FROM services WHERE id = $1`, s.ID).Scan(&stored); err != nil {
		t.Fatalf("read color: %v", err)
	}
	if s.Color != "#aa3366" || stored != "#aa3366" {
		t.Fatalf("returned %q, stored %q, want both #aa3366", s.Color, stored)
	}
}

func TestCreateServiceTrimsName(t *testing.T) {
	f := newFixture(t)
	in := validInput()
	in.Name = "  Relleno  "
	if s := f.create(t, in); s.Name != "Relleno" {
		t.Fatalf("name = %q, want Relleno", s.Name)
	}
}

func TestUpdateServiceRejectsBadFields(t *testing.T) {
	f := newFixture(t)
	s := f.create(t, validInput())
	_, err := f.m.Update(t.Context(), f.pool, s.ID, ServicePatch{Position: ptr(-1), DurationMin: ptr(0)})
	got := fieldNames(fieldsOf(t, err))
	if len(got) != 2 || got[0] != "duration_min" || got[1] != "position" {
		t.Fatalf("fields = %v, want [duration_min position]", got)
	}
}

func TestUpdateServiceRejectsOtherBadFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		patch ServicePatch
		field string
	}{
		{"empty name", ServicePatch{Name: ptr("")}, "name"},
		{"negative price", ServicePatch{PriceCents: nullable.NewNullableWithValue(int64(-1))}, "price_cents"},
		{"bad color", ServicePatch{Color: ptr("blue")}, "color"},
		{"position past the limit", ServicePatch{Position: ptr(math.MaxInt32)}, "position"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			s := f.create(t, validInput())
			_, err := f.m.Update(t.Context(), f.pool, s.ID, tc.patch)
			got := fieldNames(fieldsOf(t, err))
			if len(got) != 1 || got[0] != tc.field {
				t.Fatalf("fields = %v, want [%s]", got, tc.field)
			}
		})
	}
}

func TestUpdateServiceChangesOnlySentFields(t *testing.T) {
	f := newFixture(t)
	s := f.create(t, validInput())
	got, err := f.m.Update(t.Context(), f.pool, s.ID, ServicePatch{
		Name: ptr(" Relleno "), DurationMin: ptr(45), Color: ptr("#00FF00"), Position: ptr(7),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	want := Service{ID: s.ID, Name: "Relleno", DurationMin: 45, PriceCents: ptr(int64(25000)), Color: "#00ff00", Position: 7}
	if got.ID != want.ID || got.Name != want.Name || got.DurationMin != want.DurationMin || got.Color != want.Color ||
		got.Position != want.Position || got.Archived || got.PriceCents == nil || *got.PriceCents != *want.PriceCents {
		t.Fatalf("service = %+v, want %+v", got, want)
	}
	again, err := f.m.GetMany(t.Context(), f.pool, []uuid.UUID{s.ID})
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}
	stored := again[s.ID]
	if stored.Name != "Relleno" || stored.Color != "#00ff00" || stored.Position != 7 || stored.DurationMin != 45 {
		t.Fatalf("stored = %+v, want the patched values", stored)
	}
}

func TestUpdateServiceUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Update(t.Context(), f.pool, uuid.New(), ServicePatch{Name: ptr("X")})
	if !errors.As(err, &problem.NotFound{}) {
		t.Fatalf("err = %v, want problem.NotFound", err)
	}
}

func TestCreateServicePositions(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, validInput())
	b := f.create(t, validInput())
	if a.Position != 0 || b.Position != 1 {
		t.Fatalf("positions = %d, %d, want 0, 1", a.Position, b.Position)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].ID != a.ID || list[1].ID != b.ID {
		t.Fatalf("list = %+v, want a then b", list)
	}
}

func TestListServicesIsOrderedAndNeverNil(t *testing.T) {
	f := newFixture(t)
	empty, err := f.m.List(t.Context(), f.pool)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list = %#v, %v, want a non-nil empty slice", empty, err)
	}
	a := f.create(t, validInput())
	b := f.create(t, validInput())
	if _, err := f.m.Update(t.Context(), f.pool, a.ID, ServicePatch{Position: ptr(5)}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("list = %+v, want b (position 1) then a (position 5)", list)
	}
}

func TestCreateServiceAfterMaxPositionReturnsStoredPosition(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, validInput())
	if _, err := f.m.Update(t.Context(), f.pool, a.ID, ServicePatch{Position: ptr(math.MaxInt32 - 1)}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	b := f.create(t, validInput())
	var stored int
	if err := f.pool.QueryRow(t.Context(), `SELECT position FROM services WHERE id = $1`, b.ID).Scan(&stored); err != nil {
		t.Fatalf("read position: %v", err)
	}
	if b.Position != math.MaxInt32 || stored != b.Position {
		t.Fatalf("returned %d, stored %d, want both %d", b.Position, stored, math.MaxInt32)
	}
}

func TestUpdateServiceClearsPrice(t *testing.T) {
	f := newFixture(t)
	s := f.create(t, validInput())
	untouched, err := f.m.Update(t.Context(), f.pool, s.ID, ServicePatch{Name: ptr("Otro")})
	if err != nil || untouched.PriceCents == nil || *untouched.PriceCents != 25000 {
		t.Fatalf("price after an unrelated patch = %v, %v, want 25000", untouched.PriceCents, err)
	}
	cleared, err := f.m.Update(t.Context(), f.pool, s.ID, ServicePatch{PriceCents: nullable.NewNullNullable[int64]()})
	if err != nil || cleared.PriceCents != nil {
		t.Fatalf("price after null = %v, %v, want nil", cleared.PriceCents, err)
	}
	set, err := f.m.Update(t.Context(), f.pool, s.ID, ServicePatch{PriceCents: nullable.NewNullableWithValue(int64(0))})
	if err != nil || set.PriceCents == nil || *set.PriceCents != 0 {
		t.Fatalf("price after 0 = %v, %v, want 0", set.PriceCents, err)
	}
	stored, err := f.m.GetMany(t.Context(), f.pool, []uuid.UUID{s.ID})
	if err != nil || stored[s.ID].PriceCents == nil || *stored[s.ID].PriceCents != 0 {
		t.Fatalf("stored price = %+v, %v, want 0", stored[s.ID], err)
	}
}

func TestArchiveService(t *testing.T) {
	f := newFixture(t)
	s := f.create(t, validInput())
	archived, err := f.m.Update(t.Context(), f.pool, s.ID, ServicePatch{Archived: ptr(true)})
	if err != nil || !archived.Archived {
		t.Fatalf("archive = %+v, %v, want archived", archived, err)
	}
	f.assertArchivedAt(t, s.ID, true)
	f.clock.Advance(time.Hour)
	again, err := f.m.Update(t.Context(), f.pool, s.ID, ServicePatch{Archived: ptr(true)})
	if err != nil || !again.Archived {
		t.Fatalf("archive twice = %+v, %v, want still archived", again, err)
	}
	f.assertArchivedAt(t, s.ID, true) // still the first archive time
	restored, err := f.m.Update(t.Context(), f.pool, s.ID, ServicePatch{Archived: ptr(false)})
	if err != nil || restored.Archived {
		t.Fatalf("restore = %+v, %v, want active", restored, err)
	}
	f.assertArchivedAt(t, s.ID, false)
}

func (f fixture) assertArchivedAt(t *testing.T, id uuid.UUID, want bool) {
	t.Helper()
	var at *time.Time
	if err := f.pool.QueryRow(t.Context(), `SELECT archived_at FROM services WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatalf("read archived_at: %v", err)
	}
	if (at != nil) != want {
		t.Fatalf("archived_at = %v, want set = %v", at, want)
	}
	if want && !at.Equal(start) {
		t.Fatalf("archived_at = %v, want the clock time %v", at, start)
	}
}

func TestGetManyReturnsArchivedAndSkipsMissing(t *testing.T) {
	f := newFixture(t)
	active := f.create(t, validInput())
	gone := f.create(t, validInput())
	if _, err := f.m.Update(t.Context(), f.pool, gone.ID, ServicePatch{Archived: ptr(true)}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	missing := uuid.New()
	got, err := f.m.GetMany(t.Context(), f.pool, []uuid.UUID{active.ID, gone.ID, missing})
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}
	if len(got) != 2 || got[active.ID].Name != "Acrílico" || !got[gone.ID].Archived {
		t.Fatalf("got = %+v, want the active and the archived service only", got)
	}
	if _, ok := got[missing]; ok {
		t.Fatal("the missing id is present, want it absent")
	}
	none, err := f.m.GetMany(t.Context(), f.pool, nil)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("GetMany(nil) = %#v, %v, want an empty non-nil map", none, err)
	}
}

func TestCreateServiceDBErrorWritesNothing(t *testing.T) {
	f := newFixture(t)
	calls := 0
	// Create reads the last position (call 1), then inserts (call 2): fail the insert.
	failing := dbtest.FailingDB{DBTX: f.pool, FailAt: 2, Calls: &calls}
	_, err := f.m.Create(t.Context(), failing, validInput())
	if !errors.Is(err, dbtest.ErrInjected) {
		t.Fatalf("err = %v, want the injected database error wrapped", err)
	}
	if calls != 2 {
		t.Fatalf("database calls = %d, want 2 (the failure must hit the insert)", calls)
	}
	var n int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM services`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("services rows = %d, %v, want 0", n, err)
	}
}
