//go:build db

package appointments

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func idsOf(list []Appointment) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(list))
	for _, a := range list {
		out = append(out, a.ID)
	}
	return out
}

func wantIDs(t *testing.T, got []Appointment, want ...Appointment) {
	t.Helper()
	g, w := idsOf(got), idsOf(want)
	if len(g) != len(w) {
		t.Fatalf("ids = %v, want %v", g, w)
	}
	for i := range w {
		if g[i] != w[i] {
			t.Fatalf("ids = %v, want %v", g, w)
		}
	}
}

func (f fixture) listRange(t *testing.T, from, to time.Time, loc *time.Location) []Appointment {
	t.Helper()
	got, err := f.m.ListRange(t.Context(), f.pool, from, to, loc)
	if err != nil {
		t.Fatalf("ListRange: %v", err)
	}
	return got
}

// Review Focus 2: days are the business's local days, across the DST change too.
func TestListRangeUsesBusinessLocalDays(t *testing.T) {
	f := newFixture(t)
	madrid, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatal(err)
	}
	c := f.client(t, "Marta Solano")
	// 23:30 local on the 24th (CEST, +2) is 21:30Z.
	late24 := f.book(t, c, time.Date(2026, 10, 24, 21, 30, 0, 0, time.UTC), time.Date(2026, 10, 24, 21, 50, 0, 0, time.UTC))
	// 23:30 local on the 25th (CET, +1, after the change) is 22:30Z.
	late25 := f.book(t, c, time.Date(2026, 10, 25, 22, 30, 0, 0, time.UTC), time.Date(2026, 10, 25, 22, 50, 0, 0, time.UTC))
	// 00:30 local on the 26th is 23:30Z on the 25th.
	f.book(t, c, time.Date(2026, 10, 25, 23, 30, 0, 0, time.UTC), time.Date(2026, 10, 25, 23, 50, 0, 0, time.UTC))
	wantIDs(t, f.listRange(t, day(2026, 10, 24), day(2026, 10, 24), madrid), late24)
	wantIDs(t, f.listRange(t, day(2026, 10, 25), day(2026, 10, 25), madrid), late25)
	wantIDs(t, f.listRange(t, day(2026, 10, 24), day(2026, 10, 25), madrid), late24, late25)
}

// The spring-forward day (2026-03-29 in Europe/Madrid) has 23 hours.
func TestListRangeSpringForwardDay(t *testing.T) {
	f := newFixture(t)
	madrid, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatal(err)
	}
	c := f.client(t, "Marta Solano")
	// 23:30 local on the 28th (CET, +1) is 22:30Z.
	late28 := f.book(t, c, time.Date(2026, 3, 28, 22, 30, 0, 0, time.UTC), time.Date(2026, 3, 28, 22, 50, 0, 0, time.UTC))
	// 23:30 local on the 29th (CEST, +2, after the change) is 21:30Z.
	late29 := f.book(t, c, time.Date(2026, 3, 29, 21, 30, 0, 0, time.UTC), time.Date(2026, 3, 29, 21, 50, 0, 0, time.UTC))
	// 00:30 local on the 30th is 22:30Z on the 29th: inside a 24-hour window, outside the day.
	f.book(t, c, time.Date(2026, 3, 29, 22, 30, 0, 0, time.UTC), time.Date(2026, 3, 29, 22, 50, 0, 0, time.UTC))
	wantIDs(t, f.listRange(t, day(2026, 3, 28), day(2026, 3, 28), madrid), late28)
	wantIDs(t, f.listRange(t, day(2026, 3, 29), day(2026, 3, 29), madrid), late29)
}

func TestListRangeIncludesAllStatuses(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	a := f.service(t, "Corte", 30, ptr(2500))
	// The transitions that take a new scheduled appointment to each status.
	steps := map[Status][]Status{Scheduled: nil, Confirmed: {Confirmed}, Cancelled: {Cancelled}, NoShow: {NoShow}}
	var want []Appointment
	for i, st := range allStatuses {
		start := at.Add(time.Duration(i) * time.Hour)
		booked := f.create(t, AppointmentInput{ClientID: c, StartsAt: start, ServiceIDs: svcIDs(a.ID)})
		for _, to := range steps[st] {
			f.setStatus(t, booked.ID, to)
		}
		want = append(want, booked)
	}
	got := f.listRange(t, day(2030, 3, 4), day(2030, 3, 4), time.UTC)
	wantIDs(t, got, want...)
	for i, g := range got {
		if g.Status != allStatuses[i] || g.ClientName != "Marta Solano" {
			t.Errorf("appointment %d status %s client %q, want %s of Marta Solano", i, g.Status, g.ClientName, allStatuses[i])
		}
		wantLines(t, g.Services, lineOf(a))
	}
}

func TestListRangeEmptyIsNonNil(t *testing.T) {
	f := newFixture(t)
	got := f.listRange(t, day(2030, 3, 4), day(2030, 3, 4), time.UTC)
	if got == nil || len(got) != 0 {
		t.Fatalf("got %v, want an empty non-nil list", got)
	}
}

func TestListRangeComputesDone(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	past := f.book(t, c, at, at.Add(time.Hour))
	f.book(t, c, at.Add(2*time.Hour), at.Add(3*time.Hour))
	f.clock.Set(at.Add(90 * time.Minute))
	got := f.listRange(t, day(2030, 3, 4), day(2030, 3, 4), time.UTC)
	wantIDs(t, got[:1], past)
	if !got[0].Done || got[1].Done {
		t.Fatalf("done = %v, %v; want the ended one done and the next not", got[0].Done, got[1].Done)
	}
}

func TestListRangeToBeforeFromIs422(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.ListRange(t.Context(), f.pool, day(2030, 3, 4), day(2030, 3, 3), time.UTC)
	wantFields(t, err, fe("to", problem.MsgRangeInvalid))
}

func TestListForClientNewestFirst(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	other := f.client(t, "Valeria Mora")
	older := f.book(t, c, at, at.Add(time.Hour))
	newer := f.book(t, c, at.Add(48*time.Hour), at.Add(49*time.Hour))
	f.book(t, other, at.Add(3*time.Hour), at.Add(4*time.Hour))
	got, err := f.m.ListForClient(t.Context(), f.pool, c)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, got, newer, older)
}

func TestListForClientArchivedStillLists(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	booked := f.book(t, c, at, at.Add(time.Hour))
	f.archiveClient(t, c)
	got, err := f.m.ListForClient(t.Context(), f.pool, c)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, got, booked)
}

func TestListForClientEmptyIsNonNil(t *testing.T) {
	f := newFixture(t)
	got, err := f.m.ListForClient(t.Context(), f.pool, f.client(t, "Marta Solano"))
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want an empty non-nil list", got, err)
	}
}

func TestListForClientUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.ListForClient(t.Context(), f.pool, uuid.New())
	if !errors.As(err, new(problem.NotFound)) {
		t.Fatalf("err = %v, want problem.NotFound", err)
	}
}
