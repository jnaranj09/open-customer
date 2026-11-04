//go:build db

package appointments

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// wantOverlap checks err is an OverlapError listing exactly the given appointments, in order.
func wantOverlap(t *testing.T, err error, want ...Appointment) {
	t.Helper()
	var o OverlapError
	if !errors.As(err, &o) {
		t.Fatalf("err = %v, want OverlapError", err)
	}
	if len(o.Appointments) != len(want) {
		t.Fatalf("overlapping = %+v, want %d appointments", o.Appointments, len(want))
	}
	for i, w := range want {
		g := o.Appointments[i]
		if g.ID != w.ID || g.ClientName != w.ClientName || !g.StartsAt.Equal(w.StartsAt) || g.Services == nil {
			t.Fatalf("overlapping[%d] = %+v, want %+v", i, g, w)
		}
	}
}

func TestOverlapDetected(t *testing.T) {
	f := newFixture(t)
	marta := f.client(t, "Marta Solano")
	first := f.book(t, marta, at, at.Add(time.Hour))
	second := f.book(t, f.client(t, "Valeria Mora"), at.Add(time.Hour), at.Add(2*time.Hour))
	_, err := f.m.Create(t.Context(), f.pool, AppointmentInput{ClientID: marta, StartsAt: at.Add(30 * time.Minute), EndsAt: tm(at.Add(90 * time.Minute))})
	wantOverlap(t, err, first, second)
	if n := f.count(t, "appointments"); n != 2 {
		t.Fatalf("appointments = %d, want the overlapping one not stored", n)
	}
}

func TestBackToBackDoesNotOverlap(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	f.book(t, c, at, at.Add(time.Hour))
	f.book(t, c, at.Add(time.Hour), at.Add(2*time.Hour))
	f.book(t, c, at.Add(-time.Hour), at)
}

func TestCancelledAndNoShowDoNotCount(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	cancelled := f.book(t, c, at, at.Add(time.Hour))
	f.setStatus(t, cancelled.ID, Cancelled)
	noShow := f.book(t, c, at, at.Add(time.Hour))
	f.setStatus(t, noShow.ID, NoShow)
	f.book(t, c, at, at.Add(time.Hour))
}

func TestConfirmedCounts(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	confirmed := f.book(t, c, at, at.Add(time.Hour))
	confirmed = f.setStatus(t, confirmed.ID, Confirmed)
	_, err := f.m.Create(t.Context(), f.pool, AppointmentInput{ClientID: c, StartsAt: at, EndsAt: tm(at.Add(time.Hour))})
	wantOverlap(t, err, confirmed)
}

func TestAllowOverlapSaves(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	f.book(t, c, at, at.Add(time.Hour))
	second := f.create(t, AppointmentInput{ClientID: c, StartsAt: at.Add(2 * time.Hour), EndsAt: tm(at.Add(3 * time.Hour))})
	f.create(t, AppointmentInput{ClientID: c, StartsAt: at, EndsAt: tm(at.Add(time.Hour)), AllowOverlap: true})
	moved := f.update(t, second.ID, AppointmentPatch{StartsAt: tm(at), AllowOverlap: true})
	if !moved.StartsAt.Equal(at) {
		t.Fatalf("starts = %v, want moved onto the others", moved.StartsAt)
	}
	if n := f.count(t, "appointments"); n != 3 {
		t.Fatalf("appointments = %d, want 3", n)
	}
}

func TestUpdateOverlapOnlyWhenTimesChange(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	first := f.book(t, c, at, at.Add(time.Hour))
	second := f.create(t, AppointmentInput{ClientID: c, StartsAt: at, EndsAt: tm(at.Add(time.Hour)), AllowOverlap: true})
	f.update(t, second.ID, AppointmentPatch{Notes: nullable.NewNullableWithValue("Trae su tinte")})
	f.update(t, second.ID, AppointmentPatch{StartsAt: tm(at), EndsAt: tm(at.Add(time.Hour))})
	_, err := f.m.Update(t.Context(), f.pool, second.ID, AppointmentPatch{StartsAt: tm(at.Add(15 * time.Minute))})
	wantOverlap(t, err, first)
}

func TestUpdateServiceChangeThatMovesTheEndChecksOverlap(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	long := f.service(t, "Tinte", 120, nil)
	booked := f.book(t, c, at, at.Add(time.Hour))
	next := f.book(t, c, at.Add(time.Hour), at.Add(2*time.Hour))
	_, err := f.m.Update(t.Context(), f.pool, booked.ID, AppointmentPatch{ServiceIDs: &[]uuid.UUID{long.ID}})
	wantOverlap(t, err, next)
}

func TestUpdateExcludesSelf(t *testing.T) {
	f := newFixture(t)
	booked := f.book(t, f.client(t, "Marta Solano"), at, at.Add(time.Hour))
	got := f.update(t, booked.ID, AppointmentPatch{StartsAt: tm(at.Add(15 * time.Minute))})
	if !got.EndsAt.Equal(at.Add(75 * time.Minute)) {
		t.Fatalf("ends = %v, want moved by 15m", got.EndsAt)
	}
}

// D16: the overlap check runs only when the appointment itself is scheduled or confirmed.
func TestUpdateCancelledDoesNotCheckOverlap(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	f.book(t, c, at, at.Add(time.Hour))
	other := f.book(t, c, at.Add(2*time.Hour), at.Add(3*time.Hour))
	f.setStatus(t, other.ID, Cancelled)
	got := f.update(t, other.ID, AppointmentPatch{StartsAt: tm(at)})
	if !got.StartsAt.Equal(at) {
		t.Fatalf("starts = %v, want the cancelled one moved", got.StartsAt)
	}
}

func TestReactivateChecksOverlap(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	a := f.book(t, c, at, at.Add(time.Hour))
	f.setStatus(t, a.ID, Cancelled)
	b := f.book(t, c, at, at.Add(time.Hour))
	_, err := f.m.SetStatus(t.Context(), f.pool, a.ID, Scheduled, false)
	wantOverlap(t, err, b)
	if got := f.get(t, a.ID); got.Status != Cancelled {
		t.Fatalf("status = %s, want cancelled kept after the 409", got.Status)
	}
	got, err := f.m.SetStatus(t.Context(), f.pool, a.ID, Scheduled, true)
	if err != nil || got.Status != Scheduled {
		t.Fatalf("SetStatus with allowOverlap = %s, %v; want scheduled", got.Status, err)
	}
}

func TestStatusChangesOtherThanReactivateSkipOverlap(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Marta Solano")
	f.book(t, c, at, at.Add(time.Hour))
	second := f.create(t, AppointmentInput{ClientID: c, StartsAt: at, EndsAt: tm(at.Add(time.Hour)), AllowOverlap: true})
	if got := f.setStatus(t, second.ID, Confirmed); got.Status != Confirmed {
		t.Fatalf("status = %s, want confirmed", got.Status)
	}
}

func TestOverlapErrorIsConflict(t *testing.T) {
	err := error(OverlapError{Appointments: []Appointment{}})
	var c problem.Conflict
	if !errors.As(err, &c) || c.Name != "overlap" {
		t.Fatalf("errors.As Conflict = %+v, want Name overlap", c)
	}
	if err.Error() != "overlap" {
		t.Fatalf("Error() = %q, want overlap", err.Error())
	}
}
