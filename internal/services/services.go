// Package services holds the service catalog: what the owner offers, with its duration, price and
// color. Archiving keeps a service readable, so past appointments still show it.
package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/color"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/platform/text"
)

const (
	maxNameLen     = 80
	maxDurationMin = 1440
	// maxPosition leaves room for the "last + 1" of Create.
	maxPosition = math.MaxInt32 - 1
)

// Service is one catalog entry. PriceCents is nil when the service has no price.
type Service struct {
	ID          uuid.UUID
	Name        string
	DurationMin int
	PriceCents  *int64
	Color       string
	Position    int
	Archived    bool
}

// ServiceInput is everything Create needs.
type ServiceInput struct {
	Name        string
	DurationMin int
	PriceCents  *int64
	Color       string
}

// ServicePatch holds the fields Update changes; a nil or unspecified field is left as it is.
// PriceCents distinguishes three cases: unspecified (kept), null (cleared) and a value (set).
type ServicePatch struct {
	Name        *string
	DurationMin *int
	PriceCents  nullable.Nullable[int64]
	Color       *string
	Position    *int
	Archived    *bool
}

// Options is what serve passes to New.
type Options struct{ Clock clock.Clock }

// Module is the services module. Every method takes ctx and a db.DBTX first.
type Module struct{ clock clock.Clock }

// New returns the services module.
func New(opts Options) *Module { return &Module{clock: opts.Clock} }

// List returns every service, archived ones included, ordered by position and id. The slice is
// never nil.
func (m *Module) List(ctx context.Context, q db.DBTX) ([]Service, error) {
	rows, err := sqlc.New(q).ListServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	out := make([]Service, 0, len(rows))
	for _, r := range rows {
		out = append(out, serviceFrom(r))
	}
	return out, nil
}

// GetMany returns the services with the given ids, archived ones included, keyed by id. An id that
// matches no service is absent from the map. The map is never nil.
func (m *Module) GetMany(ctx context.Context, q db.DBTX, serviceIDs []uuid.UUID) (map[uuid.UUID]Service, error) {
	rows, err := sqlc.New(q).GetServicesByIDs(ctx, serviceIDs)
	if err != nil {
		return nil, fmt.Errorf("get services by ids: %w", err)
	}
	out := make(map[uuid.UUID]Service, len(rows))
	for _, r := range rows {
		out[r.ID] = serviceFrom(sqlc.Service(r))
	}
	return out, nil
}

// Create validates in (every bad field is reported) and places the service last.
func (m *Module) Create(ctx context.Context, q db.DBTX, in ServiceInput) (Service, error) {
	var errs []problem.FieldError
	name, ok := text.Trim(in.Name, 1, maxNameLen)
	if !ok {
		errs = append(errs, problem.FieldError{Field: "name", Message: problem.MsgNameInvalid})
	}
	duration, ok := durationMin(in.DurationMin)
	if !ok {
		errs = append(errs, problem.FieldError{Field: "duration_min", Message: problem.MsgDurationInvalid})
	}
	if in.PriceCents != nil && *in.PriceCents < 0 {
		errs = append(errs, problem.FieldError{Field: "price_cents", Message: problem.MsgPriceInvalid})
	}
	hex, ok := color.Normalize(in.Color)
	if !ok {
		errs = append(errs, problem.FieldError{Field: "color", Message: problem.MsgColorInvalid})
	}
	if len(errs) > 0 {
		return Service{}, problem.Validation{Errors: errs}
	}
	id, err := ids.New()
	if err != nil {
		return Service{}, fmt.Errorf("new service id: %w", err)
	}
	queries := sqlc.New(q)
	last, err := queries.MaxServicePosition(ctx)
	if err != nil {
		return Service{}, fmt.Errorf("max service position: %w", err)
	}
	if err := queries.InsertService(ctx, sqlc.InsertServiceParams{
		ID: id, Name: name, DurationMin: duration, PriceCents: in.PriceCents, Color: hex, Position: last + 1,
	}); err != nil {
		return Service{}, fmt.Errorf("insert service: %w", err)
	}
	return Service{
		ID: id, Name: name, DurationMin: in.DurationMin, PriceCents: in.PriceCents, Color: hex, Position: int(last + 1),
	}, nil
}

// Update changes the given fields of one service (every bad field is reported). An unknown id is
// problem.NotFound. Archiving keeps the time of the first archive.
func (m *Module) Update(ctx context.Context, q db.DBTX, id uuid.UUID, p ServicePatch) (Service, error) {
	var errs []problem.FieldError
	name := ""
	if p.Name != nil {
		var ok bool
		if name, ok = text.Trim(*p.Name, 1, maxNameLen); !ok {
			errs = append(errs, problem.FieldError{Field: "name", Message: problem.MsgNameInvalid})
		}
	}
	var duration int32
	if p.DurationMin != nil {
		var ok bool
		if duration, ok = durationMin(*p.DurationMin); !ok {
			errs = append(errs, problem.FieldError{Field: "duration_min", Message: problem.MsgDurationInvalid})
		}
	}
	if v, err := p.PriceCents.Get(); err == nil && v < 0 {
		errs = append(errs, problem.FieldError{Field: "price_cents", Message: problem.MsgPriceInvalid})
	}
	hex := ""
	if p.Color != nil {
		var ok bool
		if hex, ok = color.Normalize(*p.Color); !ok {
			errs = append(errs, problem.FieldError{Field: "color", Message: problem.MsgColorInvalid})
		}
	}
	var pos int32
	if p.Position != nil {
		var ok bool
		if pos, ok = position(*p.Position); !ok {
			errs = append(errs, problem.FieldError{Field: "position", Message: problem.MsgPositionInvalid})
		}
	}
	if len(errs) > 0 {
		return Service{}, problem.Validation{Errors: errs}
	}
	queries := sqlc.New(q)
	row, err := queries.GetService(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Service{}, problem.NotFound{}
	}
	if err != nil {
		return Service{}, fmt.Errorf("get service: %w", err)
	}
	params := sqlc.UpdateServiceParams{
		ID: id, Name: row.Name, DurationMin: row.DurationMin, PriceCents: row.PriceCents,
		Color: row.Color, Position: row.Position, ArchivedAt: row.ArchivedAt,
	}
	if p.Name != nil {
		params.Name = name
	}
	if p.DurationMin != nil {
		params.DurationMin = duration
	}
	if p.PriceCents.IsSpecified() {
		params.PriceCents = nil
		if v, err := p.PriceCents.Get(); err == nil {
			params.PriceCents = &v
		}
	}
	if p.Color != nil {
		params.Color = hex
	}
	if p.Position != nil {
		params.Position = pos
	}
	params.ArchivedAt = archivedAt(p.Archived, row.ArchivedAt, m.clock.Now())
	if err := queries.UpdateService(ctx, params); err != nil {
		return Service{}, fmt.Errorf("update service: %w", err)
	}
	return serviceFrom(sqlc.Service{
		ID: id, Name: params.Name, DurationMin: params.DurationMin, PriceCents: params.PriceCents,
		Color: params.Color, Position: params.Position, ArchivedAt: params.ArchivedAt,
	}), nil
}

// durationMin checks a requested duration and converts it to the column type.
func durationMin(n int) (int32, bool) {
	if n < 1 || n > maxDurationMin {
		return 0, false
	}
	return int32(n), true
}

// position checks a requested position and converts it to the column type.
func position(n int) (int32, bool) {
	if n < 0 || n > maxPosition {
		return 0, false
	}
	return int32(n), true
}

// archivedAt is the archive time after a patch: unchanged when archived is nil, the stored time
// (or now, when none) for true, and none for false.
func archivedAt(archived *bool, stored *time.Time, now time.Time) *time.Time {
	switch {
	case archived == nil:
		return stored
	case !*archived:
		return nil
	case stored != nil:
		return stored
	}
	return &now
}

func serviceFrom(r sqlc.Service) Service {
	return Service{
		ID: r.ID, Name: r.Name, DurationMin: int(r.DurationMin), PriceCents: r.PriceCents,
		Color: r.Color, Position: int(r.Position), Archived: r.ArchivedAt != nil,
	}
}
