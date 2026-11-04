package templates

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/archive"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/platform/text"
)

const (
	maxNameLen = 80
	maxBodyLen = 2000
)

// Template is one saved message template.
type Template struct {
	ID         uuid.UUID
	Name, Body string
	Archived   bool
}

// TemplateInput is everything Create needs.
type TemplateInput struct{ Name, Body string }

// TemplatePatch holds the fields Update changes; a nil field is left as it is.
type TemplatePatch struct {
	Name, Body *string
	Archived   *bool
}

// Options is what serve passes to New.
type Options struct {
	Clock      clock.Clock
	Attributes *attributes.Module
}

// Module is the templates module. Every method takes ctx and a db.DBTX first.
type Module struct {
	clock      clock.Clock
	attributes *attributes.Module
}

// New returns the templates module.
func New(opts Options) *Module { return &Module{clock: opts.Clock, attributes: opts.Attributes} }

// List returns every template, archived ones included, ordered by name and id. The slice is never
// nil.
func (m *Module) List(ctx context.Context, q db.DBTX) ([]Template, error) {
	rows, err := sqlc.New(q).ListTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, Template{ID: r.ID, Name: r.Name, Body: r.Body, Archived: r.ArchivedAt != nil})
	}
	return out, nil
}

// Create runs the save check (every bad field is reported) and stores the template.
func (m *Module) Create(ctx context.Context, q db.DBTX, in TemplateInput) (Template, error) {
	var errs []problem.FieldError
	name, ok := text.Trim(in.Name, 1, maxNameLen)
	if !ok {
		errs = append(errs, nameError())
	}
	if err := m.checkBody(ctx, q, in.Body); err != nil {
		if !problem.CollectFields(&errs, err) {
			return Template{}, err
		}
	}
	if len(errs) > 0 {
		return Template{}, problem.Validation{Errors: errs}
	}
	id, err := ids.New()
	if err != nil {
		return Template{}, fmt.Errorf("new template id: %w", err)
	}
	if err := sqlc.New(q).InsertTemplate(ctx, sqlc.InsertTemplateParams{ID: id, Name: name, Body: in.Body}); err != nil {
		return Template{}, fmt.Errorf("insert template: %w", err)
	}
	return Template{ID: id, Name: name, Body: in.Body}, nil
}

// Update changes the given fields of one template. The name is checked when sent and the body is
// saved through the full save check when sent. An unknown id is problem.NotFound. Archiving keeps
// the time of the first archive.
func (m *Module) Update(ctx context.Context, q db.DBTX, id uuid.UUID, p TemplatePatch) (Template, error) {
	var errs []problem.FieldError
	name := ""
	if p.Name != nil {
		var ok bool
		if name, ok = text.Trim(*p.Name, 1, maxNameLen); !ok {
			errs = append(errs, nameError())
		}
	}
	if p.Body != nil {
		if err := m.checkBody(ctx, q, *p.Body); err != nil {
			if !problem.CollectFields(&errs, err) {
				return Template{}, err
			}
		}
	}
	if len(errs) > 0 {
		return Template{}, problem.Validation{Errors: errs}
	}
	queries := sqlc.New(q)
	row, err := queries.GetTemplate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Template{}, problem.NotFound{}
	}
	if err != nil {
		return Template{}, fmt.Errorf("get template: %w", err)
	}
	params := sqlc.UpdateTemplateParams{ID: id, Name: row.Name, Body: row.Body}
	if p.Name != nil {
		params.Name = name
	}
	if p.Body != nil {
		params.Body = *p.Body
	}
	params.ArchivedAt = archive.At(p.Archived, row.ArchivedAt, m.clock.Now())
	if err := queries.UpdateTemplate(ctx, params); err != nil {
		return Template{}, fmt.Errorf("update template: %w", err)
	}
	return Template{ID: id, Name: params.Name, Body: params.Body, Archived: params.ArchivedAt != nil}, nil
}

func nameError() problem.FieldError {
	return problem.FieldError{Field: "name", Message: problem.MsgNameInvalid}
}

// checkBody is the body half of the save check. The body is not trimmed, so the length counts it
// as sent. The returned problem.Validation lists, in order: the length error, or else the Parse
// errors followed by every unknown variable in body order. Any other error is a database failure.
func (m *Module) checkBody(ctx context.Context, q db.DBTX, body string) error {
	if strings.TrimSpace(body) == "" || utf8.RuneCountInString(body) > maxBodyLen {
		return problem.Validation{Errors: []problem.FieldError{{Field: "body", Message: problem.MsgTemplateBodyInvalid}}}
	}
	parsed, parseErr := Parse(body)
	var errs []problem.FieldError
	if parseErr != nil && !problem.CollectFields(&errs, parseErr) {
		return parseErr
	}
	attrs, err := m.attributes.List(ctx, q)
	if err != nil {
		return fmt.Errorf("list attributes for the save check: %w", err)
	}
	for _, v := range parsed.Vars {
		if !known(v.NS, v.Key, attrs) {
			errs = append(errs, problem.FieldError{Field: "body", Message: fmt.Sprintf(problem.MsgTemplateUnknownVar, v.Key)})
		}
	}
	if len(errs) > 0 {
		return problem.Validation{Errors: errs}
	}
	return nil
}
