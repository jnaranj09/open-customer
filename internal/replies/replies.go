// Package replies holds the quick replies: the owner's own messages to copy into a chat, in her
// order, archived when she stops using them (reminders-and-replies design, "Quick replies").
package replies

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/archive"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/position"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/platform/text"
)

const (
	maxTitleLen = 60
	maxBodyLen  = 2000
)

// Reply is one quick reply.
type Reply struct {
	ID       uuid.UUID
	Title    string
	Body     string
	Position int
	Archived bool
}

// ReplyInput is everything Create needs.
type ReplyInput struct {
	Title string
	Body  string
}

// ReplyPatch holds the fields Update changes; a nil field is left as it is.
type ReplyPatch struct {
	Title    *string
	Body     *string
	Position *int
	Archived *bool
}

// Options is what serve passes to New.
type Options struct{ Clock clock.Clock }

// Module is the replies module. Every method takes ctx and a db.DBTX first.
type Module struct{ clock clock.Clock }

// New returns the replies module.
func New(opts Options) *Module { return &Module{clock: opts.Clock} }

// List returns every reply, archived ones included, ordered by position and id. The slice is never
// nil.
func (m *Module) List(ctx context.Context, q db.DBTX) ([]Reply, error) {
	rows, err := sqlc.New(q).ListReplies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list replies: %w", err)
	}
	out := make([]Reply, 0, len(rows))
	for _, r := range rows {
		out = append(out, replyFrom(r))
	}
	return out, nil
}

// Create validates in (every bad field is reported) and places the reply last.
func (m *Module) Create(ctx context.Context, q db.DBTX, in ReplyInput) (Reply, error) {
	var errs []problem.FieldError
	title := checkTitle(&errs, in.Title)
	body := checkBody(&errs, in.Body)
	if len(errs) > 0 {
		return Reply{}, problem.Validation{Errors: errs}
	}
	id, err := ids.New()
	if err != nil {
		return Reply{}, fmt.Errorf("new reply id: %w", err)
	}
	queries := sqlc.New(q)
	last, err := queries.MaxReplyPosition(ctx)
	if err != nil {
		return Reply{}, fmt.Errorf("max reply position: %w", err)
	}
	next, ok := position.Next(last)
	if !ok {
		return Reply{}, problem.Field("position", problem.MsgPositionInvalid)
	}
	if err := queries.InsertReply(ctx, sqlc.InsertReplyParams{ID: id, Title: title, Body: body, Position: next}); err != nil {
		return Reply{}, fmt.Errorf("insert reply: %w", err)
	}
	return Reply{ID: id, Title: title, Body: body, Position: int(next)}, nil
}

// Update changes the given fields of one reply (every bad field is reported). An unknown id is
// problem.NotFound. A position change sets that one row; the web moves the others. Archiving keeps
// the time of the first archive.
func (m *Module) Update(ctx context.Context, q db.DBTX, id uuid.UUID, p ReplyPatch) (Reply, error) {
	queries := sqlc.New(q)
	row, err := queries.GetReply(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reply{}, problem.NotFound{}
	}
	if err != nil {
		return Reply{}, fmt.Errorf("get reply: %w", err)
	}
	params := sqlc.UpdateReplyParams{ID: id, Title: row.Title, Body: row.Body, Position: row.Position}
	var errs []problem.FieldError
	if p.Title != nil {
		params.Title = checkTitle(&errs, *p.Title)
	}
	if p.Body != nil {
		params.Body = checkBody(&errs, *p.Body)
	}
	if p.Position != nil {
		var ok bool
		if params.Position, ok = position.Check(*p.Position); !ok {
			errs = append(errs, problem.FieldError{Field: "position", Message: problem.MsgPositionInvalid})
		}
	}
	if len(errs) > 0 {
		return Reply{}, problem.Validation{Errors: errs}
	}
	params.ArchivedAt = archive.At(p.Archived, row.ArchivedAt, m.clock.Now())
	if err := queries.UpdateReply(ctx, params); err != nil {
		return Reply{}, fmt.Errorf("update reply: %w", err)
	}
	return replyFrom(sqlc.QuickReply{
		ID: id, Title: params.Title, Body: params.Body, Position: params.Position, ArchivedAt: params.ArchivedAt,
	}), nil
}

// checkTitle is a title through text.Line: one line of 1 to 60 characters.
func checkTitle(errs *[]problem.FieldError, raw string) string {
	title, msg := text.Line(raw, maxTitleLen)
	if msg == "" && title == nil {
		msg = problem.MsgReplyTitleRequired
	}
	if msg != "" {
		*errs = append(*errs, problem.FieldError{Field: "title", Message: msg})
		return ""
	}
	return *title
}

// checkBody trims a body at both ends (line breaks inside are kept) and checks its length.
func checkBody(errs *[]problem.FieldError, raw string) string {
	body := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(body); n < 1 || n > maxBodyLen {
		*errs = append(*errs, problem.FieldError{Field: "body", Message: problem.MsgReplyBodyLength})
	}
	return body
}

func replyFrom(r sqlc.QuickReply) Reply {
	return Reply{ID: r.ID, Title: r.Title, Body: r.Body, Position: int(r.Position), Archived: r.ArchivedAt != nil}
}
