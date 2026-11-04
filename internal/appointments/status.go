package appointments

import (
	"fmt"
	"time"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// ParseStatus returns the status named s, or a 422 on "status".
func ParseStatus(s string) (Status, error) {
	switch st := Status(s); st {
	case Scheduled, Confirmed, Cancelled, NoShow:
		return st, nil
	}
	return "", problem.Field("status", problem.MsgStatusInvalid)
}

// IsDone reports whether an appointment that is still active has ended by now. A cancelled or
// no-show appointment is never done.
func IsDone(status Status, endsAt, now time.Time) bool {
	switch status {
	case Scheduled, Confirmed:
		return !endsAt.After(now)
	case Cancelled, NoShow:
		return false
	}
	return false
}

// Transition moves a to the status to, or returns a 409 invalid-transition. It sets Status,
// UpdatedAt and Done; the caller stores the result.
func Transition(a Appointment, to Status, now time.Time) (Appointment, error) {
	if !allowed(a.Status, to) {
		return a, fmt.Errorf("transition %s to %s: %w", a.Status, to, problem.Conflict{Name: "invalid-transition"})
	}
	a.Status = to
	a.UpdatedAt = now
	a.Done = IsDone(to, a.EndsAt, now)
	return a, nil
}

func allowed(from, to Status) bool {
	switch from {
	case Scheduled:
		switch to {
		case Confirmed, Cancelled, NoShow:
			return true
		case Scheduled:
			return false
		}
	case Confirmed:
		switch to {
		case Cancelled, NoShow:
			return true
		case Scheduled, Confirmed:
			return false
		}
	case Cancelled, NoShow:
		switch to {
		case Scheduled:
			return true
		case Confirmed, Cancelled, NoShow:
			return false
		}
	}
	return false
}
