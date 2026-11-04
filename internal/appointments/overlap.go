package appointments

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// OverlapError says other active appointments overlap the times of a write. It unwraps to the
// "overlap" conflict; the handler adds Appointments to the response.
type OverlapError struct{ Appointments []Appointment }

// Error implements error.
func (OverlapError) Error() string { return "overlap" }

// Unwrap returns the problem the handler answers with.
func (OverlapError) Unwrap() error { return problem.Conflict{Name: "overlap"} }

// Overlaps returns the scheduled and confirmed appointments other than exclude that overlap
// [start, end), ordered by start. Back-to-back appointments do not overlap. The check takes no
// lock (D16): two writes at the same moment can both pass it.
func (m *Module) Overlaps(ctx context.Context, q db.DBTX, start, end time.Time, exclude uuid.UUID) ([]Appointment, error) {
	rows, err := sqlc.New(q).ListOverlapping(ctx, sqlc.ListOverlappingParams{StartsAt: start, EndsAt: end, Exclude: exclude})
	if err != nil {
		return nil, fmt.Errorf("list overlapping appointments: %w", err)
	}
	list := make([]sqlc.GetAppointmentRow, 0, len(rows))
	for _, r := range rows {
		list = append(list, sqlc.GetAppointmentRow(r))
	}
	return m.withLines(ctx, q, list)
}

// checkOverlap returns an OverlapError when Overlaps finds any appointment.
func (m *Module) checkOverlap(ctx context.Context, q db.DBTX, start, end time.Time, exclude uuid.UUID) error {
	found, err := m.Overlaps(ctx, q, start, end, exclude)
	if err != nil {
		return err
	}
	if len(found) > 0 {
		return OverlapError{Appointments: found}
	}
	return nil
}

// counts reports whether an appointment with status s takes part in the overlap check.
func counts(s Status) bool {
	switch s {
	case Scheduled, Confirmed:
		return true
	case Cancelled, NoShow:
		return false
	}
	return false
}
