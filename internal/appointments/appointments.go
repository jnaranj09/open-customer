// Package appointments holds appointments: their status rules, the service lines copied onto them,
// the overlap check and the stored operations. Done is computed on read, never stored.
package appointments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/platform/text"
	"github.com/jnaranj09/open-customer/internal/services"
)

// Status is where an appointment stands. Done is not a status: see IsDone.
type Status string

// The four statuses.
const (
	Scheduled Status = "scheduled"
	Confirmed Status = "confirmed"
	Cancelled Status = "cancelled"
	NoShow    Status = "no_show"
)

// Line is one service copied onto an appointment at save time. PriceCents is nil when the service
// had no price.
type Line struct {
	ServiceID   uuid.UUID
	Name        string
	PriceCents  *int64
	DurationMin int
}

// Appointment is one booking. Done is computed on read, never stored. TotalCents is the sum of the
// line prices, computed and stored when the lines are saved. Services is never nil.
type Appointment struct {
	ID, ClientID uuid.UUID
	ClientName   string
	StartsAt     time.Time
	EndsAt       time.Time
	Status       Status
	Done         bool
	TotalCents   int64
	PaidCents    *int64
	Attrs        json.RawMessage
	Notes        *string
	Services     []Line
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AppointmentInput is everything Create needs. A nil EndsAt means StartsAt plus the services'
// durations. A nil or blank Notes is stored as NULL.
type AppointmentInput struct {
	ClientID     uuid.UUID
	StartsAt     time.Time
	EndsAt       *time.Time
	ServiceIDs   []uuid.UUID
	Attrs        json.RawMessage
	Notes        *string
	AllowOverlap bool
}

// AppointmentPatch holds the fields Update changes; a nil or unspecified field is left as it is. A
// zero StartsAt or EndsAt is a 422. ServiceIDs replaces the whole line list. Attrs is merged by
// key; nil means absent. Notes and PaidCents distinguish unspecified (kept), null (cleared) and a
// value (set).
type AppointmentPatch struct {
	StartsAt, EndsAt *time.Time
	ServiceIDs       *[]uuid.UUID
	Attrs            json.RawMessage
	Notes            nullable.Nullable[string]
	PaidCents        nullable.Nullable[int64]
	AllowOverlap     bool
}

// Options is what serve passes to New.
type Options struct {
	Clock      clock.Clock
	Services   *services.Module
	Attributes *attributes.Module
}

// Module is the appointments module. Every method takes ctx and a db.DBTX first.
type Module struct {
	clock    clock.Clock
	services *services.Module
	attrs    *attributes.Module
}

// New returns the appointments module.
func New(opts Options) *Module {
	return &Module{clock: opts.Clock, services: opts.Services, attrs: opts.Attributes}
}

// Get returns one appointment with its lines. An unknown id is problem.NotFound.
func (m *Module) Get(ctx context.Context, q db.DBTX, id uuid.UUID) (Appointment, error) {
	r, err := sqlc.New(q).GetAppointment(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Appointment{}, problem.NotFound{}
	}
	if err != nil {
		return Appointment{}, fmt.Errorf("get appointment: %w", err)
	}
	list, err := m.withLines(ctx, q, []sqlc.GetAppointmentRow{r})
	if err != nil {
		return Appointment{}, err
	}
	return list[0], nil
}

// Create validates in (every bad field is reported), checks for an overlap unless AllowOverlap,
// and stores the appointment as scheduled with its lines copied from the current services.
func (m *Module) Create(ctx context.Context, q db.DBTX, in AppointmentInput) (Appointment, error) {
	queries := sqlc.New(q)
	var errs []problem.FieldError
	if err := activeClient(ctx, queries, in.ClientID); err != nil && !problem.CollectFields(&errs, err) {
		return Appointment{}, err
	}
	zero := zeroTimes(&errs, &in.StartsAt, in.EndsAt)
	found, err := m.services.GetMany(ctx, q, in.ServiceIDs)
	if err != nil {
		return Appointment{}, fmt.Errorf("get appointment services: %w", err)
	}
	lines, lerr := planLines(in.ServiceIDs, found, nil)
	problem.CollectFields(&errs, lerr)
	end, known := time.Time{}, true
	switch {
	case in.EndsAt != nil:
		end = *in.EndsAt
	case lerr != nil:
		known = false // the service_ids error is already reported
	case len(lines) == 0:
		errs = append(errs, field("service_ids", problem.MsgServicesOrEndRequired))
		known = false
	default:
		end = in.StartsAt.Add(durationOf(lines))
	}
	if known && !zero && !end.After(in.StartsAt) {
		errs = append(errs, field("ends_at", problem.MsgEndBeforeStart))
	}
	attrs, err := m.attrs.Validate(ctx, q, attributes.ScopeAppointment, nil, in.Attrs, true)
	if err != nil && !problem.CollectFields(&errs, err) {
		return Appointment{}, fmt.Errorf("validate appointment attrs: %w", err)
	}
	notes, ok := text.Notes(in.Notes)
	if !ok {
		errs = append(errs, field("notes", problem.MsgNotesTooLong))
	}
	if len(errs) > 0 {
		return Appointment{}, problem.Validation{Errors: errs}
	}
	if !in.AllowOverlap {
		if err := m.checkOverlap(ctx, q, in.StartsAt, end, uuid.Nil); err != nil {
			return Appointment{}, err
		}
	}
	id, err := ids.New()
	if err != nil {
		return Appointment{}, fmt.Errorf("new appointment id: %w", err)
	}
	if err := queries.InsertAppointment(ctx, sqlc.InsertAppointmentParams{
		ID: id, ClientID: in.ClientID, StartsAt: in.StartsAt, EndsAt: end, TotalCents: totalCents(lines),
		Attrs: attrs, Notes: notes, Now: m.clock.Now(),
	}); err != nil {
		return Appointment{}, fmt.Errorf("insert appointment: %w", err)
	}
	if err := insertLines(ctx, queries, id, lines); err != nil {
		return Appointment{}, err
	}
	return m.Get(ctx, q, id)
}

// Update changes the given fields of one appointment (every bad field is reported), also on past
// and done appointments. ServiceIDs keeps the saved line of a service already on it and copies
// only added ones at today's price (D10); it recomputes total_cents but never paid_cents. Without EndsAt, a new service list sets ends_at to start plus its durations,
// and a moved start moves ends_at by the same amount. The overlap check runs only when the times
// change and the appointment is scheduled or confirmed. An unknown id is problem.NotFound.
func (m *Module) Update(ctx context.Context, q db.DBTX, id uuid.UUID, p AppointmentPatch) (Appointment, error) {
	cur, err := m.Get(ctx, q, id)
	if err != nil {
		return Appointment{}, err
	}
	var errs []problem.FieldError
	zero := zeroTimes(&errs, p.StartsAt, p.EndsAt)
	start := cur.StartsAt
	if p.StartsAt != nil {
		start = *p.StartsAt
	}
	lines, total := cur.Services, cur.TotalCents
	var lerr error
	if p.ServiceIDs != nil {
		found, err := m.services.GetMany(ctx, q, *p.ServiceIDs)
		if err != nil {
			return Appointment{}, fmt.Errorf("get appointment services: %w", err)
		}
		lines, lerr = planLines(*p.ServiceIDs, found, cur.Services)
		problem.CollectFields(&errs, lerr)
		total = totalCents(lines)
	}
	end, known := cur.EndsAt, true
	switch {
	case p.EndsAt != nil:
		end = *p.EndsAt
	case p.ServiceIDs != nil && lerr != nil:
		known = false // the service_ids error is already reported
	case p.ServiceIDs != nil && len(lines) == 0:
		errs = append(errs, field("service_ids", problem.MsgServicesOrEndRequired))
		known = false
	case p.ServiceIDs != nil:
		end = start.Add(durationOf(lines))
	case p.StartsAt != nil:
		end = cur.EndsAt.Add(start.Sub(cur.StartsAt))
	}
	if known && !zero && !end.After(start) {
		errs = append(errs, field("ends_at", problem.MsgEndBeforeStart))
	}
	attrs := cur.Attrs
	if p.Attrs != nil {
		merged, err := m.attrs.Validate(ctx, q, attributes.ScopeAppointment, cur.Attrs, p.Attrs, false)
		if err != nil && !problem.CollectFields(&errs, err) {
			return Appointment{}, fmt.Errorf("validate appointment attrs: %w", err)
		}
		attrs = merged
	}
	paid := cur.PaidCents
	if p.PaidCents.IsSpecified() {
		paid = nullval.Ptr(p.PaidCents)
		if paid != nil && *paid < 0 {
			errs = append(errs, field("paid_cents", problem.MsgPaidInvalid))
		}
	}
	notes := cur.Notes
	if p.Notes.IsSpecified() {
		var ok bool
		if notes, ok = text.Notes(nullval.Ptr(p.Notes)); !ok {
			errs = append(errs, field("notes", problem.MsgNotesTooLong))
		}
	}
	if len(errs) > 0 {
		return Appointment{}, problem.Validation{Errors: errs}
	}
	timesChanged := !start.Equal(cur.StartsAt) || !end.Equal(cur.EndsAt)
	if timesChanged && counts(cur.Status) && !p.AllowOverlap {
		if err := m.checkOverlap(ctx, q, start, end, id); err != nil {
			return Appointment{}, err
		}
	}
	queries := sqlc.New(q)
	if err := queries.UpdateAppointment(ctx, sqlc.UpdateAppointmentParams{
		ID: id, StartsAt: start, EndsAt: end, TotalCents: total, PaidCents: paid, Attrs: attrs,
		Notes: notes, Now: m.clock.Now(),
	}); err != nil {
		return Appointment{}, fmt.Errorf("update appointment: %w", err)
	}
	if p.ServiceIDs != nil {
		if err := queries.DeleteAppointmentServices(ctx, id); err != nil {
			return Appointment{}, fmt.Errorf("delete appointment services: %w", err)
		}
		if err := insertLines(ctx, queries, id, lines); err != nil {
			return Appointment{}, err
		}
	}
	return m.Get(ctx, q, id)
}

// SetStatus moves one appointment to the status to (see Transition). A change to scheduled
// ("Reactivar") runs the overlap check unless allowOverlap. An unknown id is problem.NotFound.
func (m *Module) SetStatus(ctx context.Context, q db.DBTX, id uuid.UUID, to Status, allowOverlap bool) (Appointment, error) {
	a, err := m.Get(ctx, q, id)
	if err != nil {
		return Appointment{}, err
	}
	now := m.clock.Now()
	moved, err := Transition(a, to, now)
	if err != nil {
		return Appointment{}, err
	}
	if to == Scheduled && !allowOverlap {
		if err := m.checkOverlap(ctx, q, a.StartsAt, a.EndsAt, id); err != nil {
			return Appointment{}, err
		}
	}
	if err := sqlc.New(q).UpdateAppointmentStatus(ctx, sqlc.UpdateAppointmentStatusParams{
		ID: id, Status: string(moved.Status), Now: now,
	}); err != nil {
		return Appointment{}, fmt.Errorf("update appointment status: %w", err)
	}
	return m.Get(ctx, q, id)
}

// ListRange returns the appointments of every status that start on the local days from to to
// (both inclusive, civil dates read in loc), ordered by start. to before from is a 422 on "to".
// There is no length limit here: the endpoint sets its own, and bootstrap reads a longer window.
func (m *Module) ListRange(ctx context.Context, q db.DBTX, from, to time.Time, loc *time.Location) ([]Appointment, error) {
	fy, fm, fd := from.Date()
	ty, tm, td := to.Date()
	first := time.Date(fy, fm, fd, 0, 0, 0, 0, loc)
	last := time.Date(ty, tm, td, 0, 0, 0, 0, loc)
	if last.Before(first) {
		return nil, problem.Field("to", problem.MsgRangeInvalid)
	}
	rows, err := sqlc.New(q).ListAppointmentsInRange(ctx, sqlc.ListAppointmentsInRangeParams{
		RangeStart: first, RangeEnd: locale.AddDays(last, 1, loc),
	})
	if err != nil {
		return nil, fmt.Errorf("list appointments in range: %w", err)
	}
	list := make([]sqlc.GetAppointmentRow, 0, len(rows))
	for _, r := range rows {
		list = append(list, sqlc.GetAppointmentRow(r))
	}
	return m.withLines(ctx, q, list)
}

// ListForClient returns one client's appointments, newest first. An unknown client is
// problem.NotFound; an archived one still lists.
func (m *Module) ListForClient(ctx context.Context, q db.DBTX, clientID uuid.UUID) ([]Appointment, error) {
	queries := sqlc.New(q)
	_, err := queries.GetClientState(ctx, clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, problem.NotFound{}
	}
	if err != nil {
		return nil, fmt.Errorf("get client state: %w", err)
	}
	rows, err := queries.ListAppointmentsForClient(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("list client appointments: %w", err)
	}
	list := make([]sqlc.GetAppointmentRow, 0, len(rows))
	for _, r := range rows {
		list = append(list, sqlc.GetAppointmentRow(r))
	}
	return m.withLines(ctx, q, list)
}

// zeroTimes adds a 422 on starts_at or ends_at for each given time that is the zero time (a
// missing or blank value decodes to it) and reports whether it found one.
func zeroTimes(errs *[]problem.FieldError, start, end *time.Time) bool {
	zero := false
	if start != nil && start.IsZero() {
		*errs = append(*errs, field("starts_at", problem.MsgNullNotAllowed))
		zero = true
	}
	if end != nil && end.IsZero() {
		*errs = append(*errs, field("ends_at", problem.MsgNullNotAllowed))
		zero = true
	}
	return zero
}

// activeClient returns a 422 on client_id when the client is unknown or archived.
func activeClient(ctx context.Context, queries *sqlc.Queries, id uuid.UUID) error {
	archivedAt, err := queries.GetClientState(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && archivedAt != nil) {
		return problem.Field("client_id", problem.MsgClientUnavailable)
	}
	if err != nil {
		return fmt.Errorf("get client state: %w", err)
	}
	return nil
}

// insertLines stores lines in order, at positions 0 to n-1.
func insertLines(ctx context.Context, queries *sqlc.Queries, id uuid.UUID, lines []Line) error {
	var pos int16
	for _, l := range lines {
		if err := queries.InsertAppointmentService(ctx, sqlc.InsertAppointmentServiceParams{
			AppointmentID: id, Position: pos, ServiceID: l.ServiceID, Name: l.Name,
			PriceCents: l.PriceCents, DurationMin: int64(l.DurationMin),
		}); err != nil {
			return fmt.Errorf("insert appointment service: %w", err)
		}
		pos++
	}
	return nil
}

// withLines loads the lines of rows with one query and builds the appointments, in row order,
// with Done computed from the clock. The slice is never nil.
func (m *Module) withLines(ctx context.Context, q db.DBTX, rows []sqlc.GetAppointmentRow) ([]Appointment, error) {
	out := make([]Appointment, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	byID := make(map[uuid.UUID][]Line, len(rows))
	keys := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		byID[r.ID] = []Line{}
		keys = append(keys, r.ID)
	}
	lines, err := sqlc.New(q).ListAppointmentServices(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("list appointment services: %w", err)
	}
	for _, l := range lines {
		byID[l.AppointmentID] = append(byID[l.AppointmentID], Line{
			ServiceID: l.ServiceID, Name: l.Name, PriceCents: l.PriceCents, DurationMin: int(l.DurationMin),
		})
	}
	now := m.clock.Now()
	for _, r := range rows {
		status := Status(r.Status)
		out = append(out, Appointment{
			ID: r.ID, ClientID: r.ClientID, ClientName: r.ClientName, StartsAt: r.StartsAt, EndsAt: r.EndsAt,
			Status: status, Done: IsDone(status, r.EndsAt, now), TotalCents: r.TotalCents,
			PaidCents: r.PaidCents, Attrs: r.Attrs, Notes: r.Notes, Services: byID[r.ID],
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

// field is one field error of a write.
func field(name, msg string) problem.FieldError { return problem.FieldError{Field: name, Message: msg} }
