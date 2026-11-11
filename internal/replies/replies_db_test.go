//go:build db

package replies

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/position"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// Quick replies (reminders-and-replies design, "Quick replies"; item 57).

var start = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

type fixture struct {
	pool  *pgxpool.Pool
	clock *clock.Fake
	m     *Module
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	fake := clock.NewFake(start)
	return fixture{pool: dbtest.Pool(t), clock: fake, m: New(Options{Clock: fake})}
}

func ptr[T any](v T) *T { return &v }

func (f fixture) create(t *testing.T, title string) Reply {
	t.Helper()
	r, err := f.m.Create(t.Context(), f.pool, ReplyInput{Title: title, Body: "Hola, ¿cómo estás?"})
	if err != nil {
		t.Fatalf("Create %q: %v", title, err)
	}
	return r
}

func (f fixture) list(t *testing.T) []Reply {
	t.Helper()
	got, err := f.m.List(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return got
}

func fieldsOf(t *testing.T, err error) []problem.FieldError {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, want problem.Validation", err)
	}
	return v.Errors
}

func TestCreateAppendsAtTheEnd(t *testing.T) {
	f := newFixture(t)
	a, b := f.create(t, "Dirección"), f.create(t, "Precios")
	if a.Position != 0 || b.Position != 1 {
		t.Fatalf("positions = %d, %d, want 0 and 1", a.Position, b.Position)
	}
	got := f.list(t)
	if len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID {
		t.Fatalf("list = %+v, want a then b", got)
	}
}

func TestListIsNeverNil(t *testing.T) {
	f := newFixture(t)
	if got := f.list(t); got == nil || len(got) != 0 {
		t.Fatalf("list = %#v, want an empty non-nil slice", got)
	}
}

func TestCreateTrimsAndKeepsInnerLineBreaks(t *testing.T) {
	f := newFixture(t)
	r, err := f.m.Create(t.Context(), f.pool, ReplyInput{Title: "  Dirección ", Body: "\n Calle 1\nSan José \n"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.Title != "Dirección" || r.Body != "Calle 1\nSan José" {
		t.Fatalf("reply = %+v, want trimmed title and body with the inner line break", r)
	}
	if got := f.list(t); got[0].Title != r.Title || got[0].Body != r.Body {
		t.Fatalf("stored = %+v, want %+v", got[0], r)
	}
}

func TestCreateRejects(t *testing.T) {
	cases := []struct {
		name  string
		in    ReplyInput
		field string
		msg   string
	}{
		{"a blank title", ReplyInput{Title: "  ", Body: "Hola"}, "title", problem.MsgReplyTitleRequired},
		{"a title with a line break", ReplyInput{Title: "Uno\nDos", Body: "Hola"}, "title", problem.MsgTextOneLine},
		{"a title over 60 characters", ReplyInput{Title: strings.Repeat("ñ", 61), Body: "Hola"}, "title", problem.MsgTextTooLong},
		{"a blank body", ReplyInput{Title: "Hola", Body: " \n "}, "body", problem.MsgReplyBodyLength},
		{"a body over 2000 characters", ReplyInput{Title: "Hola", Body: strings.Repeat("ñ", 2001)}, "body", problem.MsgReplyBodyLength},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.m.Create(t.Context(), f.pool, c.in)
			got := fieldsOf(t, err)
			if len(got) != 1 || got[0].Field != c.field || got[0].Message != c.msg {
				t.Fatalf("errors = %+v, want %s: %s", got, c.field, c.msg)
			}
		})
	}
}

func TestCreateAcceptsTheLimits(t *testing.T) {
	f := newFixture(t)
	if _, err := f.m.Create(t.Context(), f.pool, ReplyInput{Title: strings.Repeat("ñ", 60), Body: strings.Repeat("ñ", 2000)}); err != nil {
		t.Fatalf("60 and 2000 runes: %v", err)
	}
}

func TestCreateCollectsEveryError(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(t.Context(), f.pool, ReplyInput{})
	if got := fieldsOf(t, err); len(got) != 2 || got[0].Field != "title" || got[1].Field != "body" {
		t.Fatalf("errors = %+v, want title and body", got)
	}
}

func TestCreateAfterMaxPositionIs422(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Uno")
	if _, err := f.m.Update(t.Context(), f.pool, a.ID, ReplyPatch{Position: ptr(position.Max)}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	_, err := f.m.Create(t.Context(), f.pool, ReplyInput{Title: "Dos", Body: "Hola"})
	if got := fieldsOf(t, err); len(got) != 1 || got[0].Field != "position" {
		t.Fatalf("errors = %+v, want one on position", got)
	}
}

func TestUpdatePositionSetsOneRow(t *testing.T) {
	f := newFixture(t)
	a, b := f.create(t, "Uno"), f.create(t, "Dos")
	moved, err := f.m.Update(t.Context(), f.pool, b.ID, ReplyPatch{Position: ptr(0)})
	if err != nil || moved.Position != 0 {
		t.Fatalf("moved = %+v, %v, want position 0", moved, err)
	}
	got := f.list(t)
	if got[0].Position != 0 || got[1].Position != 0 || got[0].ID == got[1].ID || (got[0].ID != a.ID && got[0].ID != b.ID) {
		t.Fatalf("list = %+v, want both at 0: only the sent row changes", got)
	}
}

func TestUpdateChangesOnlySentFields(t *testing.T) {
	f := newFixture(t)
	r := f.create(t, "Uno")
	got, err := f.m.Update(t.Context(), f.pool, r.ID, ReplyPatch{Body: ptr(" Nuevo texto ")})
	if err != nil || got.Title != "Uno" || got.Body != "Nuevo texto" || got.Position != 0 {
		t.Fatalf("updated = %+v, %v, want the new body only", got, err)
	}
	got, err = f.m.Update(t.Context(), f.pool, r.ID, ReplyPatch{Title: ptr("Otro")})
	if err != nil || got.Title != "Otro" || got.Body != "Nuevo texto" {
		t.Fatalf("updated = %+v, %v, want the new title only", got, err)
	}
}

func TestUpdateRejects(t *testing.T) {
	f := newFixture(t)
	r := f.create(t, "Uno")
	_, err := f.m.Update(t.Context(), f.pool, r.ID, ReplyPatch{Title: ptr(""), Body: ptr(""), Position: ptr(-1)})
	got := fieldsOf(t, err)
	if len(got) != 3 || got[0].Field != "title" || got[1].Field != "body" || got[2].Field != "position" {
		t.Fatalf("errors = %+v, want title, body and position", got)
	}
}

func TestUpdateUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	if _, err := f.m.Update(t.Context(), f.pool, uuid.New(), ReplyPatch{Title: ptr("")}); !errors.As(err, new(problem.NotFound)) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestArchiveAndRestore(t *testing.T) {
	f := newFixture(t)
	r := f.create(t, "Uno")
	archived, err := f.m.Update(t.Context(), f.pool, r.ID, ReplyPatch{Archived: ptr(true)})
	if err != nil || !archived.Archived || !f.list(t)[0].Archived {
		t.Fatalf("archive = %+v, %v, want archived and listed", archived, err)
	}
	restored, err := f.m.Update(t.Context(), f.pool, r.ID, ReplyPatch{Archived: ptr(false)})
	if err != nil || restored.Archived || f.list(t)[0].Archived {
		t.Fatalf("restore = %+v, %v, want active", restored, err)
	}
}

// TestDBErrors fails each database call of each operation in turn: the caller gets the database
// error back wrapped, never a success.
func TestDBErrors(t *testing.T) {
	ops := map[string]func(t *testing.T, f fixture, q db.DBTX) error{
		"List": func(t *testing.T, f fixture, q db.DBTX) error {
			t.Helper()
			_, err := f.m.List(t.Context(), q)
			return err
		},
		"Create": func(t *testing.T, f fixture, q db.DBTX) error {
			t.Helper()
			_, err := f.m.Create(t.Context(), q, ReplyInput{Title: "Uno", Body: "Hola"})
			return err
		},
		"Update": func(t *testing.T, f fixture, q db.DBTX) error {
			t.Helper()
			r := f.create(t, "Uno")
			_, err := f.m.Update(t.Context(), q, r.ID, ReplyPatch{Archived: ptr(true)})
			return err
		},
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			steps := 0
			clean := newFixture(t)
			if err := op(t, clean, dbtest.FailingDB{DBTX: clean.pool, Calls: &steps}); err != nil {
				t.Fatalf("clean run: %v", err)
			}
			for step := 1; step <= steps; step++ {
				f := newFixture(t)
				calls := 0
				err := op(t, f, dbtest.FailingDB{DBTX: f.pool, FailAt: step, Calls: &calls})
				if !errors.Is(err, dbtest.ErrInjected) {
					t.Fatalf("failure at step %d of %d: err = %v", step, steps, err)
				}
			}
		})
	}
}
