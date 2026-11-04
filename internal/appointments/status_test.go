package appointments

import (
	"errors"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var allStatuses = []Status{Scheduled, Confirmed, Cancelled, NoShow}

func TestTransitionAllowed(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	cases := []struct{ from, to Status }{
		{Scheduled, Confirmed},
		{Scheduled, Cancelled},
		{Confirmed, Cancelled},
		{Scheduled, NoShow},
		{Confirmed, NoShow},
		{Cancelled, Scheduled},
		{NoShow, Scheduled},
	}
	for _, c := range cases {
		t.Run(string(c.from)+"->"+string(c.to), func(t *testing.T) {
			a := Appointment{Status: c.from, EndsAt: now.Add(-time.Hour)}
			got, err := Transition(a, c.to, now)
			if err != nil {
				t.Fatalf("Transition: %v", err)
			}
			if got.Status != c.to {
				t.Errorf("Status = %s, want %s", got.Status, c.to)
			}
			if !got.UpdatedAt.Equal(now) {
				t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, now)
			}
			wantDone := IsDone(c.to, a.EndsAt, now)
			if got.Done != wantDone {
				t.Errorf("Done = %v, want %v", got.Done, wantDone)
			}
		})
	}
}

func TestTransitionSetsDoneFromTheNewStatus(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	past := Appointment{Status: Scheduled, EndsAt: now.Add(-time.Hour)}
	confirmed, err := Transition(past, Confirmed, now)
	if err != nil || !confirmed.Done {
		t.Fatalf("confirmed past: done=%v err=%v, want done", confirmed.Done, err)
	}
	cancelled, err := Transition(Appointment{Status: Scheduled, EndsAt: now.Add(-time.Hour), Done: true}, Cancelled, now)
	if err != nil || cancelled.Done {
		t.Fatalf("cancelled past: done=%v err=%v, want not done", cancelled.Done, err)
	}
}

func TestTransitionRejected(t *testing.T) {
	allowed := map[[2]Status]bool{
		{Scheduled, Confirmed}: true, {Scheduled, Cancelled}: true, {Confirmed, Cancelled}: true,
		{Scheduled, NoShow}: true, {Confirmed, NoShow}: true, {Cancelled, Scheduled}: true,
		{NoShow, Scheduled}: true,
	}
	var rejected [][2]Status
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			if !allowed[[2]Status{from, to}] {
				rejected = append(rejected, [2]Status{from, to})
			}
		}
	}
	if len(rejected) != 9 {
		t.Fatalf("test table has %d rejected pairs, want 9", len(rejected))
	}
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, p := range rejected {
		t.Run(string(p[0])+"->"+string(p[1]), func(t *testing.T) {
			a := Appointment{Status: p[0], UpdatedAt: now.Add(-time.Hour)}
			got, err := Transition(a, p[1], now)
			var c problem.Conflict
			if !errors.As(err, &c) || c.Name != "invalid-transition" {
				t.Fatalf("err = %v, want Conflict invalid-transition", err)
			}
			if got.Status != p[0] && got.Status != "" {
				t.Errorf("returned status changed to %s", got.Status)
			}
		})
	}
}

func TestIsDoneAtTheEnd(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	if !IsDone(Scheduled, now, now) {
		t.Error("scheduled with ends_at == now must be done")
	}
	if !IsDone(Confirmed, now.Add(-time.Second), now) {
		t.Error("confirmed that ended a second ago must be done")
	}
	if IsDone(Scheduled, now.Add(time.Second), now) {
		t.Error("scheduled that ends a second from now must not be done")
	}
}

func TestIsDoneNeverForCancelledOrNoShow(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, s := range []Status{Cancelled, NoShow} {
		if IsDone(s, now.Add(-time.Hour), now) {
			t.Errorf("%s must never be done", s)
		}
	}
}

func TestParseStatusAccepted(t *testing.T) {
	for _, s := range allStatuses {
		got, err := ParseStatus(string(s))
		if err != nil || got != s {
			t.Errorf("ParseStatus(%q) = %q, %v", s, got, err)
		}
	}
}

func TestParseStatusRejected(t *testing.T) {
	for _, in := range []string{"done", "completed", "", "Scheduled"} {
		_, err := ParseStatus(in)
		var v problem.Validation
		if !errors.As(err, &v) || len(v.Errors) != 1 ||
			v.Errors[0].Field != "status" || v.Errors[0].Message != problem.MsgStatusInvalid {
			t.Errorf("ParseStatus(%q) err = %v, want 422 on status", in, err)
		}
	}
}
