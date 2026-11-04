// Package clients holds the people the owner serves: name, phone, notes and the client-scope
// attribute values. Archiving keeps a client readable, so past appointments still show it.
package clients

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
	"github.com/jnaranj09/open-customer/internal/platform/archive"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/platform/text"
)

const maxNameLen = 120

// Client is one stored client. PhoneE164, PhoneDigits and Notes are nil when unset.
type Client struct {
	ID          uuid.UUID
	Name        string
	PhoneE164   *string
	PhoneDigits *string
	Notes       *string
	Attrs       json.RawMessage
	Archived    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ClientInput is everything Create needs. An empty or nil Phone means no phone.
type ClientInput struct {
	Name                string
	Phone, Notes        *string
	Attrs               json.RawMessage
	AllowDuplicatePhone bool
}

// ClientPatch holds the fields Update changes. Phone and Notes distinguish unspecified (kept),
// null or "" (cleared) and a value (set). Attrs is merged by key; nil means absent.
type ClientPatch struct {
	Name                *string
	Phone, Notes        nullable.Nullable[string]
	Attrs               json.RawMessage
	Archived            *bool
	AllowDuplicatePhone bool
}

// DuplicatePhoneError says other active clients already have the phone. It unwraps to the
// "duplicate-phone" conflict; the handler adds Clients to the response.
type DuplicatePhoneError struct{ Clients []Client }

// Error implements error.
func (DuplicatePhoneError) Error() string { return "duplicate phone" }

// Unwrap returns the problem the handler answers with.
func (DuplicatePhoneError) Unwrap() error { return problem.Conflict{Name: "duplicate-phone"} }

// Options is what serve passes to New.
type Options struct {
	Clock      clock.Clock
	Attributes *attributes.Module
}

// Module is the clients module. Every method takes ctx and a db.DBTX first.
type Module struct {
	clock clock.Clock
	attrs *attributes.Module
}

// New returns the clients module.
func New(opts Options) *Module { return &Module{clock: opts.Clock, attrs: opts.Attributes} }

// List returns the archived or the active clients, ordered by name (ignoring case). The slice is
// never nil.
func (m *Module) List(ctx context.Context, q db.DBTX, archived bool) ([]Client, error) {
	rows, err := sqlc.New(q).ListClients(ctx, archived)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	return clientsFrom(rows), nil
}

// Get returns one client, archived or not. An unknown id is problem.NotFound.
func (m *Module) Get(ctx context.Context, q db.DBTX, id uuid.UUID) (Client, error) {
	row, err := sqlc.New(q).GetClient(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Client{}, problem.NotFound{}
	}
	if err != nil {
		return Client{}, fmt.Errorf("get client: %w", err)
	}
	return clientFrom(row), nil
}

// Create validates in (every bad field is reported), warns about a phone another active client
// has (unless AllowDuplicatePhone) and stores the client. region is the business country code.
func (m *Module) Create(ctx context.Context, q db.DBTX, in ClientInput, region string) (Client, error) {
	var errs []problem.FieldError
	name, ok := text.Trim(in.Name, 1, maxNameLen)
	if !ok {
		errs = append(errs, problem.FieldError{Field: "name", Message: problem.MsgClientNameInvalid})
	}
	e164, digits, perr := phoneFields(in.Phone, region)
	problem.CollectFields(&errs, perr)
	notes, ok := text.Notes(in.Notes)
	if !ok {
		errs = append(errs, notesTooLong)
	}
	attrs, aerr := m.attrs.Validate(ctx, q, attributes.ScopeClient, nil, in.Attrs, true)
	if aerr != nil && !problem.CollectFields(&errs, aerr) {
		return Client{}, fmt.Errorf("validate client attrs: %w", aerr)
	}
	if len(errs) > 0 {
		return Client{}, problem.Validation{Errors: errs}
	}
	id, err := ids.New()
	if err != nil {
		return Client{}, fmt.Errorf("new client id: %w", err)
	}
	queries := sqlc.New(q)
	if e164 != nil && !in.AllowDuplicatePhone {
		if err := checkDuplicate(ctx, queries, *e164, id); err != nil {
			return Client{}, err
		}
	}
	now := m.clock.Now()
	if err := queries.InsertClient(ctx, sqlc.InsertClientParams{
		ID: id, Name: name, PhoneE164: e164, PhoneDigits: digits, Notes: notes, Attrs: attrs, Now: now,
	}); err != nil {
		return Client{}, fmt.Errorf("insert client: %w", err)
	}
	return clientFrom(sqlc.Client{
		ID: id, Name: name, PhoneE164: e164, PhoneDigits: digits, Notes: notes, Attrs: attrs,
		CreatedAt: now, UpdatedAt: now,
	}), nil
}

// Update changes the given fields of one client (every bad field is reported). An unknown id is
// problem.NotFound. The duplicate-phone check runs when the phone changes and when an archived
// client is restored.
func (m *Module) Update(ctx context.Context, q db.DBTX, id uuid.UUID, p ClientPatch, region string) (Client, error) {
	queries := sqlc.New(q)
	row, err := queries.GetClient(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Client{}, problem.NotFound{}
	}
	if err != nil {
		return Client{}, fmt.Errorf("get client: %w", err)
	}
	now := m.clock.Now()
	params := sqlc.UpdateClientParams{
		ID: id, Name: row.Name, PhoneE164: row.PhoneE164, PhoneDigits: row.PhoneDigits, Notes: row.Notes,
		Attrs: row.Attrs, Now: now, ArchivedAt: archive.At(p.Archived, row.ArchivedAt, now),
	}
	var errs []problem.FieldError
	if p.Name != nil {
		var ok bool
		if params.Name, ok = text.Trim(*p.Name, 1, maxNameLen); !ok {
			errs = append(errs, problem.FieldError{Field: "name", Message: problem.MsgClientNameInvalid})
		}
	}
	if p.Phone.IsSpecified() {
		e164, digits, perr := phoneFields(nullval.Ptr(p.Phone), region)
		problem.CollectFields(&errs, perr)
		params.PhoneE164, params.PhoneDigits = e164, digits
	}
	if p.Notes.IsSpecified() {
		notes, ok := text.Notes(nullval.Ptr(p.Notes))
		if !ok {
			errs = append(errs, notesTooLong)
		}
		params.Notes = notes
	}
	if p.Attrs != nil {
		attrs, aerr := m.attrs.Validate(ctx, q, attributes.ScopeClient, row.Attrs, p.Attrs, false)
		if aerr != nil && !problem.CollectFields(&errs, aerr) {
			return Client{}, fmt.Errorf("validate client attrs: %w", aerr)
		}
		params.Attrs = attrs
	}
	if len(errs) > 0 {
		return Client{}, problem.Validation{Errors: errs}
	}
	restoring := row.ArchivedAt != nil && params.ArchivedAt == nil
	if (phoneChanged(row.PhoneE164, params.PhoneE164) || restoring) && params.PhoneE164 != nil && !p.AllowDuplicatePhone {
		if err := checkDuplicate(ctx, queries, *params.PhoneE164, id); err != nil {
			return Client{}, err
		}
	}
	if err := queries.UpdateClient(ctx, params); err != nil {
		return Client{}, fmt.Errorf("update client: %w", err)
	}
	return clientFrom(sqlc.Client{
		ID: id, Name: params.Name, PhoneE164: params.PhoneE164, PhoneDigits: params.PhoneDigits,
		Notes: params.Notes, Attrs: params.Attrs, CreatedAt: row.CreatedAt, UpdatedAt: params.Now,
		ArchivedAt: params.ArchivedAt,
	}), nil
}

// checkDuplicate returns a DuplicatePhoneError when another active client has e164.
func checkDuplicate(ctx context.Context, queries *sqlc.Queries, e164 string, self uuid.UUID) error {
	rows, err := queries.ListActiveClientsByPhone(ctx, sqlc.ListActiveClientsByPhoneParams{PhoneE164: e164, ID: self})
	if err != nil {
		return fmt.Errorf("list clients by phone: %w", err)
	}
	if len(rows) > 0 {
		return DuplicatePhoneError{Clients: clientsFrom(rows)}
	}
	return nil
}

// notesTooLong is the notes error of Create and Update.
var notesTooLong = problem.FieldError{Field: "notes", Message: problem.MsgNotesTooLong}

// phoneFields normalizes an optional phone; nil or "" is no phone.
func phoneFields(raw *string, region string) (e164, digits *string, err error) {
	if raw == nil || *raw == "" {
		return nil, nil, nil
	}
	e, d, err := NormalizePhone(*raw, region)
	if err != nil {
		return nil, nil, err
	}
	return &e, &d, nil
}

func phoneChanged(stored, next *string) bool {
	if stored == nil || next == nil {
		return stored != next
	}
	return *stored != *next
}

func clientsFrom(rows []sqlc.Client) []Client {
	out := make([]Client, 0, len(rows))
	for _, r := range rows {
		out = append(out, clientFrom(r))
	}
	return out
}

func clientFrom(r sqlc.Client) Client {
	return Client{
		ID: r.ID, Name: r.Name, PhoneE164: r.PhoneE164, PhoneDigits: r.PhoneDigits, Notes: r.Notes,
		Attrs: r.Attrs, Archived: r.ArchivedAt != nil, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
