//go:build db

package settings

import (
	"errors"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var start = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

func TestCreateThenGet(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	created, err := m.Create(t.Context(), pool, validInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := m.Get(t.Context(), pool)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := Settings{
		Name: "Made Up Studio", Timezone: "Europe/Madrid", Locale: "es", Currency: "EUR",
		CountryCode: "ES", AccentColor: "#aa3366", PushHour: 8,
	}
	if got != want || created != want {
		t.Errorf("got %+v, created %+v, want %+v", got, created, want)
	}
}

func TestCreateRejectsBadInputWithoutWriting(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	in := validInput()
	in.Locale = "en"
	if _, err := m.Create(t.Context(), pool, in); !errors.As(err, &problem.Validation{}) {
		t.Fatalf("Create err = %v, want Validation", err)
	}
	if _, err := m.Get(t.Context(), pool); !errors.As(err, &problem.NotFound{}) {
		t.Errorf("Get err = %v, want NotFound (nothing written)", err)
	}
}

func TestGetBeforeSetupIsNotFound(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	if _, err := m.Get(t.Context(), pool); !errors.As(err, &problem.NotFound{}) {
		t.Errorf("err = %v, want NotFound", err)
	}
}

func TestUpdateBeforeSetupIsNotFound(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	name := "x"
	if _, err := m.Update(t.Context(), pool, SettingsPatch{Name: &name}); !errors.As(err, &problem.NotFound{}) {
		t.Errorf("err = %v, want NotFound", err)
	}
}

func TestUpdateChangesOnlyGivenFields(t *testing.T) {
	pool := dbtest.Pool(t)
	fake := clock.NewFake(start)
	m := New(Options{Clock: fake, MediaDir: t.TempDir()})
	if _, err := m.Create(t.Context(), pool, validInput()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	later := start.Add(time.Hour)
	fake.Set(later)
	hour := 19
	got, err := m.Update(t.Context(), pool, SettingsPatch{PushHour: &hour})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	want := Settings{
		Name: "Made Up Studio", Timezone: "Europe/Madrid", Locale: "es", Currency: "EUR",
		CountryCode: "ES", AccentColor: "#aa3366", PushHour: 19,
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	var updatedAt time.Time
	if err := pool.QueryRow(t.Context(), "SELECT updated_at FROM business_settings").Scan(&updatedAt); err != nil {
		t.Fatalf("read updated_at: %v", err)
	}
	if !updatedAt.Equal(later) {
		t.Errorf("updated_at = %v, want %v", updatedAt, later)
	}
}

func TestUpdateRejectsBadAccent(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	if _, err := m.Create(t.Context(), pool, validInput()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	bad := "red"
	if _, err := m.Update(t.Context(), pool, SettingsPatch{AccentColor: &bad}); !errors.As(err, &problem.Validation{}) {
		t.Fatalf("err = %v, want Validation", err)
	}
	got, err := m.Get(t.Context(), pool)
	if err != nil || got.AccentColor != "#aa3366" {
		t.Errorf("accent = %q, %v; want unchanged #aa3366", got.AccentColor, err)
	}
}

func TestCreateTwiceFails(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	if _, err := m.Create(t.Context(), pool, validInput()); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	second := validInput()
	second.Name = "Other"
	if _, err := m.Create(t.Context(), pool, second); err == nil {
		t.Fatal("second Create succeeded, want an error")
	}
	var n int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM business_settings").Scan(&n); err != nil || n != 1 {
		t.Errorf("rows = %d, %v; want 1", n, err)
	}
	if got, _ := m.Get(t.Context(), pool); got.Name != "Made Up Studio" {
		t.Errorf("name = %q, want the first row kept", got.Name)
	}
}
