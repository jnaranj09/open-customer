package reminders

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// Action is what the owner did with a due item.
type Action string

// The two actions.
const (
	ActionSent    Action = "sent"
	ActionSkipped Action = "skipped"
)

// ParseAction parses an action from a request body. Anything else is a 422 on the field "action".
func ParseAction(s string) (Action, error) {
	switch a := Action(s); a {
	case ActionSent, ActionSkipped:
		return a, nil
	}
	return "", problem.Field("action", problem.MsgActionInvalid)
}

// phoneDigitsPattern is the digits of an international number: 1 to 15, nothing else.
var phoneDigitsPattern = regexp.MustCompile(`^[0-9]{1,15}$`)

// LogEntry is one thing the owner did with a due item. AnchorAt is the appointment start for a
// "before" rule and nil for a come_back rule. Body and PhoneDigits are exactly what was sent, and
// nil for a skip that never had them.
type LogEntry struct {
	RuleID        uuid.UUID
	AppointmentID uuid.UUID
	AnchorAt      *time.Time
	Action        Action
	Body          *string
	PhoneDigits   *string
	ActedAt       time.Time
}

// LogItem is a stored log row, with the rule's kind and the client it belongs to.
type LogItem struct {
	LogEntry
	ID         uuid.UUID
	Kind       Kind
	ClientID   uuid.UUID
	ClientName string
}

// Record stores e and reports created true. A rule archived since the item was shown still
// records: the owner acted on what she saw. When the same item (rule, appointment and anchor) is
// already logged, nothing is written and the stored row comes back with created false. Every bad
// field is reported as a 422: an unknown rule or appointment, an anchor that does not fit the
// rule's kind, and phone digits that are not 1 to 15 digits.
func (m *Module) Record(ctx context.Context, q db.DBTX, e LogEntry) (LogItem, bool, error) {
	if _, err := ParseAction(string(e.Action)); err != nil {
		return LogItem{}, false, err
	}
	queries := sqlc.New(q)
	var errs []problem.FieldError
	kind, err := queries.GetRuleKind(ctx, e.RuleID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		errs = append(errs, problem.FieldError{Field: "rule_id", Message: problem.MsgRuleUnknown})
	case err != nil:
		return LogItem{}, false, fmt.Errorf("get rule kind: %w", err)
	default:
		if (Kind(kind) == KindBefore) != (e.AnchorAt != nil) {
			errs = append(errs, problem.FieldError{Field: "anchor_at", Message: problem.MsgAnchorInvalid})
		}
	}
	known, err := queries.GetAppointmentExists(ctx, e.AppointmentID)
	if err != nil {
		return LogItem{}, false, fmt.Errorf("get appointment: %w", err)
	}
	if !known {
		errs = append(errs, problem.FieldError{Field: "appointment_id", Message: problem.MsgAppointmentUnknown})
	}
	if e.PhoneDigits != nil && !phoneDigitsPattern.MatchString(*e.PhoneDigits) {
		errs = append(errs, problem.FieldError{Field: "phone_digits", Message: problem.MsgPhoneDigitsInvalid})
	}
	if len(errs) > 0 {
		return LogItem{}, false, problem.Validation{Errors: errs}
	}
	id, err := ids.New()
	if err != nil {
		return LogItem{}, false, fmt.Errorf("new log id: %w", err)
	}
	stored, err := queries.InsertLog(ctx, sqlc.InsertLogParams{
		ID: id, RuleID: e.RuleID, AppointmentID: e.AppointmentID, AnchorAt: e.AnchorAt,
		Action: string(e.Action), Body: e.Body, PhoneDigits: e.PhoneDigits, ActedAt: e.ActedAt,
	})
	created := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		// The conflict path: the item is already logged, so return that row (O4).
		stored, err = queries.GetLogByKey(ctx, sqlc.GetLogByKeyParams{
			RuleID: e.RuleID, AppointmentID: e.AppointmentID, AnchorAt: e.AnchorAt,
		})
	}
	if err != nil {
		return LogItem{}, false, fmt.Errorf("insert log: %w", err)
	}
	row, err := queries.GetLogItem(ctx, stored)
	if err != nil {
		return LogItem{}, false, fmt.Errorf("get log item: %w", err)
	}
	return logItemFrom(row), created, nil
}

// LogForClient returns the client's log, newest acted_at first. An unknown client is
// problem.NotFound. The slice is never nil.
func (m *Module) LogForClient(ctx context.Context, q db.DBTX, clientID uuid.UUID) ([]LogItem, error) {
	queries := sqlc.New(q)
	known, err := queries.GetClientExists(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("get client: %w", err)
	}
	if !known {
		return nil, problem.NotFound{}
	}
	rows, err := queries.ListLogForClient(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("list log for client: %w", err)
	}
	out := make([]LogItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, logItemFrom(sqlc.GetLogItemRow(r)))
	}
	return out, nil
}

// LogForDate returns the rows acted on during the local day of day in tz, newest first. The slice
// is never nil.
func (m *Module) LogForDate(ctx context.Context, q db.DBTX, day time.Time, tz *time.Location) ([]LogItem, error) {
	from := locale.LocalDay(day, tz)
	rows, err := sqlc.New(q).ListLogInRange(ctx, sqlc.ListLogInRangeParams{
		FromAt: from, ToAt: locale.AddDays(from, 1, tz),
	})
	if err != nil {
		return nil, fmt.Errorf("list log for date: %w", err)
	}
	out := make([]LogItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, logItemFrom(sqlc.GetLogItemRow(r)))
	}
	return out, nil
}

func logItemFrom(r sqlc.GetLogItemRow) LogItem {
	return LogItem{
		LogEntry: LogEntry{
			RuleID: r.RuleID, AppointmentID: r.AppointmentID, AnchorAt: r.AnchorAt, Action: Action(r.Action),
			Body: r.Body, PhoneDigits: r.PhoneDigits, ActedAt: r.ActedAt,
		},
		ID: r.ID, Kind: Kind(r.Kind), ClientID: r.ClientID, ClientName: r.ClientName,
	}
}
