package reminders

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// NextReminder is a client's next planned message (reminders-and-replies design, "Look-ahead
// pass"): its due day (business-local midnight), the services of the appointment it comes from, and
// whether a general rule makes it.
type NextReminder struct {
	DueDate        time.Time
	SourceServices []string
	General        bool
}

// NextForClient is the client's earliest reminder due today or late, or else her earliest one after
// today (come-back from her done visits, "before" from her bookings in the next 365 days); ties by
// due day, then rule id. Nil when there is none. An unknown client is problem.NotFound; an archived
// one is allowed and has none.
func (m *Module) NextForClient(ctx context.Context, q db.DBTX, now time.Time, clientID uuid.UUID) (*NextReminder, error) {
	exists, err := sqlc.New(q).GetClientExists(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("get client exists: %w", err)
	}
	if !exists {
		return nil, problem.NotFound{}
	}
	d, err := m.loadDue(ctx, q, now, loadOpts{client: uuid.NullUUID{UUID: clientID, Valid: true}})
	if err != nil {
		return nil, err
	}
	horizon := locale.AddDays(locale.LocalDay(now, d.TZ), maxOffsetDays, d.TZ)
	for _, items := range [][]DueItem{compute(d, now), upcoming(d, now, horizon)} {
		items = slices.DeleteFunc(items, func(it DueItem) bool { return it.ClientID != clientID })
		if len(items) == 0 {
			continue
		}
		it := slices.MinFunc(items, func(x, y DueItem) int {
			return cmp.Or(x.DueDate.Compare(y.DueDate), slices.Compare(x.RuleID[:], y.RuleID[:]))
		})
		return &NextReminder{DueDate: it.DueDate, SourceServices: it.SourceServices, General: it.General}, nil
	}
	return nil, nil
}
