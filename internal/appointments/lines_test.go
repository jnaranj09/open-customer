package appointments

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/services"
)

func ptr(n int64) *int64 { return &n }

func svc(name string, dur int, price *int64, archived bool) services.Service {
	return services.Service{ID: uuid.New(), Name: name, DurationMin: dur, PriceCents: price, Archived: archived}
}

func byID(ss ...services.Service) map[uuid.UUID]services.Service {
	m := make(map[uuid.UUID]services.Service, len(ss))
	for _, s := range ss {
		m[s.ID] = s
	}
	return m
}

func assertServiceIDsError(t *testing.T, err error, msg string) {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) || len(v.Errors) != 1 ||
		v.Errors[0].Field != "service_ids" || v.Errors[0].Message != msg {
		t.Fatalf("err = %v, want 422 on service_ids %q", err, msg)
	}
}

func TestPlanLinesCopiesAndRepeats(t *testing.T) {
	a := svc("Corte", 30, ptr(2500), false)
	b := svc("Tinte", 90, nil, false)
	got, err := planLines([]uuid.UUID{a.ID, b.ID, a.ID}, byID(a, b), nil)
	if err != nil {
		t.Fatalf("planLines: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3", len(got))
	}
	want := []Line{
		{ServiceID: a.ID, Name: "Corte", PriceCents: ptr(2500), DurationMin: 30},
		{ServiceID: b.ID, Name: "Tinte", PriceCents: nil, DurationMin: 90},
		{ServiceID: a.ID, Name: "Corte", PriceCents: ptr(2500), DurationMin: 30},
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.ServiceID != w.ServiceID || g.Name != w.Name || g.DurationMin != w.DurationMin ||
			(g.PriceCents == nil) != (w.PriceCents == nil) ||
			(g.PriceCents != nil && *g.PriceCents != *w.PriceCents) {
			t.Errorf("line %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestPlanLinesEmptyIsNonNil(t *testing.T) {
	got, err := planLines(nil, nil, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want empty non-nil slice", got, err)
	}
}

func TestPlanLinesUnknownAndArchivedRejected(t *testing.T) {
	active := svc("Corte", 30, ptr(2500), false)
	archived := svc("Viejo", 30, ptr(100), true)
	found := byID(active, archived)
	for name, ids := range map[string][]uuid.UUID{
		"unknown":  {active.ID, uuid.New()},
		"archived": {active.ID, archived.ID},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := planLines(ids, found, nil)
			assertServiceIDsError(t, err, problem.MsgServiceUnavailable)
		})
	}
}

func TestPlanLinesKeepsArchivedExistingLine(t *testing.T) {
	archived := svc("Nombre nuevo", 45, ptr(9900), true)
	keep := []Line{{ServiceID: archived.ID, Name: "Nombre viejo", PriceCents: ptr(5000), DurationMin: 30}}
	got, err := planLines([]uuid.UUID{archived.ID}, byID(archived), keep)
	if err != nil {
		t.Fatalf("planLines: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Nombre viejo" || got[0].DurationMin != 30 ||
		got[0].PriceCents == nil || *got[0].PriceCents != 5000 {
		t.Fatalf("got %+v, want the kept line unchanged", got)
	}
	_, err = planLines([]uuid.UUID{archived.ID, archived.ID}, byID(archived), keep)
	assertServiceIDsError(t, err, problem.MsgServiceUnavailable)
}

func TestPlanLinesUnknownIDRejectedEvenIfKept(t *testing.T) {
	// An ID missing from found is an error even when a kept line has it.
	keep := []Line{{ServiceID: uuid.New(), Name: "X", DurationMin: 10}}
	_, err := planLines([]uuid.UUID{keep[0].ServiceID}, nil, keep)
	assertServiceIDsError(t, err, problem.MsgServiceUnavailable)
}

func TestPlanLinesKeptTwiceArchivedGivesTwoLines(t *testing.T) {
	archived := svc("Viejo", 30, ptr(100), true)
	keep := []Line{
		{ServiceID: archived.ID, Name: "A", DurationMin: 30},
		{ServiceID: archived.ID, Name: "B", DurationMin: 20},
	}
	got, err := planLines([]uuid.UUID{archived.ID, archived.ID}, byID(archived), keep)
	if err != nil || len(got) != 2 || got[0].Name != "A" || got[1].Name != "B" {
		t.Fatalf("got %+v, %v; want kept lines A then B in order", got, err)
	}
}

// Ruling F12: an active service already on the appointment is copied again at the current price.
func TestPlanLinesRecopiesActiveServiceAlreadyKept(t *testing.T) {
	active := svc("Corte", 40, ptr(3000), false)
	keep := []Line{{ServiceID: active.ID, Name: "Corte viejo", PriceCents: ptr(1000), DurationMin: 20}}
	got, err := planLines([]uuid.UUID{active.ID}, byID(active), keep)
	if err != nil {
		t.Fatalf("planLines: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Corte" || got[0].DurationMin != 40 ||
		got[0].PriceCents == nil || *got[0].PriceCents != 3000 {
		t.Fatalf("got %+v, want a fresh copy at the current price", got)
	}
}

func TestPlanLinesTooManyServices(t *testing.T) {
	a := svc("Corte", 30, nil, false)
	ids := make([]uuid.UUID, 21)
	for i := range ids {
		ids[i] = a.ID
	}
	_, err := planLines(ids, byID(a), nil)
	assertServiceIDsError(t, err, problem.MsgTooManyServices)
	if _, err := planLines(ids[:20], byID(a), nil); err != nil {
		t.Fatalf("20 ids must be accepted: %v", err)
	}
}

func TestTotalAndDurationSkipNullPrice(t *testing.T) {
	lines := []Line{
		{PriceCents: ptr(2500), DurationMin: 30},
		{PriceCents: nil, DurationMin: 90},
		{PriceCents: ptr(1000), DurationMin: 15},
	}
	if got := totalCents(lines); got != 3500 {
		t.Errorf("totalCents = %d, want 3500", got)
	}
	if got := durationOf(lines); got != 135*time.Minute {
		t.Errorf("durationOf = %v, want 2h15m", got)
	}
	if totalCents(nil) != 0 || durationOf(nil) != 0 {
		t.Error("empty lines must total 0")
	}
}
