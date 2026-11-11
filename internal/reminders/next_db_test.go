//go:build db

package reminders

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// The client's next message (reminders-and-replies design, "Look-ahead pass", CUJ R10; item 42).

func (e dueEnv) nextAt(t *testing.T, now time.Time, clientID uuid.UUID) *NextReminder {
	t.Helper()
	got, err := e.m.NextForClient(t.Context(), e.pool, now, clientID)
	if err != nil {
		t.Fatalf("NextForClient: %v", err)
	}
	return got
}

func wantNextReminder(t *testing.T, got *NextReminder, due time.Time, general bool, services ...string) {
	t.Helper()
	if got == nil || !got.DueDate.Equal(due) || got.General != general || !slices.Equal(got.SourceServices, services) {
		t.Fatalf("next = %+v, want %s general=%v %v", got, due.Format(time.DateOnly), general, services)
	}
}

func TestCUJR10NextMessageFutureTodayLate(t *testing.T) {
	e := newDueEnv(t)
	unas := e.service(t, "Uñas", 60, nil)
	e.general(t, 30)
	ana := e.client(t, "Ana Rojas", true)
	e.book(t, ana.ID, e.at(2026, 10, 1, 10, 0), unas)
	due := e.at(2026, 10, 31, 0, 0)

	for _, now := range []time.Time{e.at(2026, 10, 20, 9, 0), e.at(2026, 10, 31, 9, 0), e.at(2026, 11, 5, 9, 0)} {
		wantNextReminder(t, e.nextAt(t, now, ana.ID), due, true, "Uñas")
	}
}

func TestNextMessageNoneIsNil(t *testing.T) {
	e := newDueEnv(t)
	ana := e.client(t, "Ana Rojas", true)
	if got := e.nextAt(t, e.at(2026, 10, 20, 9, 0), ana.ID); got != nil {
		t.Fatalf("next = %+v, want none", got)
	}
}

func TestNextMessageUnknownClientIsNotFound(t *testing.T) {
	e := newDueEnv(t)
	if _, err := e.m.NextForClient(t.Context(), e.pool, e.at(2026, 10, 20, 9, 0), uuid.New()); !errors.As(err, new(problem.NotFound)) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestNextMessageBeforeReminderFromABooking(t *testing.T) {
	e := newDueEnv(t)
	unas := e.service(t, "Uñas", 60, nil)
	e.rule(t, RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: e.template(t, "Aviso", "Hola")})
	ana := e.client(t, "Ana Rojas", true)
	e.book(t, ana.ID, e.at(2026, 11, 20, 10, 0), unas) // 51 days ahead: beyond the due list's window

	wantNextReminder(t, e.nextAt(t, e.at(2026, 9, 30, 11, 0), ana.ID), e.at(2026, 11, 19, 0, 0), false, "Uñas")
}

// TestLookAheadDBErrors fails each database call of the preview and the next message in turn.
func TestLookAheadDBErrors(t *testing.T) {
	setup := func(t *testing.T) (dueEnv, Rule, uuid.UUID) {
		t.Helper()
		e := newDueEnv(t)
		unas := e.service(t, "Uñas", 60, nil)
		g := e.general(t, 30)
		ana := e.client(t, "Ana Rojas", true)
		e.book(t, ana.ID, e.at(2026, 10, 1, 10, 0), unas)
		return e, g, ana.ID
	}
	t.Run("RulePreview", func(t *testing.T) {
		sweepDBErrors(t, func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
			t.Helper()
			e, g, _ := setup(t)
			return e.pool, func(q db.DBTX) error {
				_, err := e.m.RulePreview(t.Context(), q, e.at(2026, 10, 20, 9, 0), RuleDraft{RuleID: nu(g.ID), OffsetDays: 30, TemplateID: g.TemplateID})
				return err
			}
		})
	})
	t.Run("NextForClient", func(t *testing.T) {
		sweepDBErrors(t, func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
			t.Helper()
			e, _, ana := setup(t)
			return e.pool, func(q db.DBTX) error {
				_, err := e.m.NextForClient(t.Context(), q, e.at(2026, 10, 20, 9, 0), ana)
				return err
			}
		})
	})
}
