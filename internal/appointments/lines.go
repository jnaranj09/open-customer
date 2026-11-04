package appointments

import (
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/services"
)

// maxServices is the most service lines one appointment may have.
const maxServices = 20

// planLines turns the requested service IDs into lines, in order. keep is the appointment's
// current lines (nil on create). An active service is copied again at its current name, price and
// duration. An archived service is allowed only while a kept line still has it, and that line is
// reused as it is (D10). An unknown ID, or an archived one with no unused kept line, is a 422 on
// service_ids. The result is never nil.
func planLines(ids []uuid.UUID, found map[uuid.UUID]services.Service, keep []Line) ([]Line, error) {
	if len(ids) > maxServices {
		return nil, serviceIDsError(problem.MsgTooManyServices)
	}
	used := make([]bool, len(keep))
	out := make([]Line, 0, len(ids))
	for _, id := range ids {
		s, ok := found[id]
		if !ok {
			return nil, serviceIDsError(problem.MsgServiceUnavailable)
		}
		if !s.Archived {
			out = append(out, Line{ServiceID: s.ID, Name: s.Name, PriceCents: s.PriceCents, DurationMin: s.DurationMin})
			continue
		}
		i := firstUnused(keep, used, id)
		if i < 0 {
			return nil, serviceIDsError(problem.MsgServiceUnavailable)
		}
		used[i] = true
		out = append(out, keep[i])
	}
	return out, nil
}

// firstUnused returns the index of the first kept line for id that is not used yet, or -1.
func firstUnused(keep []Line, used []bool, id uuid.UUID) int {
	for i, l := range keep {
		if !used[i] && l.ServiceID == id {
			return i
		}
	}
	return -1
}

func serviceIDsError(msg string) error {
	return problem.Validation{Errors: []problem.FieldError{{Field: "service_ids", Message: msg}}}
}

// totalCents sums the line prices; a line without a price adds nothing.
func totalCents(lines []Line) int64 {
	var sum int64
	for _, l := range lines {
		if l.PriceCents != nil {
			sum += *l.PriceCents
		}
	}
	return sum
}

// durationOf sums the line durations.
func durationOf(lines []Line) time.Duration {
	var sum time.Duration
	for _, l := range lines {
		sum += time.Duration(l.DurationMin) * time.Minute
	}
	return sum
}
