// Package scheduler is the daily timer (design doc "Process model", item 58). Every minute it
// writes a heartbeat and, once per business-local day between the push hour and 20:00, sends one
// Web Push with the number of due reminders. It owns the scheduler_state table.
package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/push"
	"github.com/jnaranj09/open-customer/internal/reminders"
	"github.com/jnaranj09/open-customer/internal/settings"
)

// The scheduler_state keys.
const (
	keyHeartbeat       = "heartbeat"
	keyPushCheckedDate = "push_checked_date"
	keyLastPush        = "last_push"
)

const (
	// pushEndHour is the business-local hour from which no daily push goes out.
	pushEndHour = 20
	// dateLayout is a business-local day, "YYYY-MM-DD".
	dateLayout = "2006-01-02"
	// pushURL is the screen the notification opens ("Para enviar hoy").
	pushURL = "/enviar"
)

// Sender sends one payload to every push subscription. *push.Module is the real one; it is an
// interface only so the tests can use a fake push service.
type Sender interface {
	SendAll(ctx context.Context, q db.DBTX, p push.Payload) (push.SendResult, error)
}

// Options is what serve passes to New.
type Options struct {
	Clock     clock.Clock
	Pool      *pgxpool.Pool
	Settings  *settings.Module
	Reminders *reminders.Module
	Push      Sender
}

// Scheduler is the daily timer.
type Scheduler struct {
	clock     clock.Clock
	pool      *pgxpool.Pool
	settings  *settings.Module
	reminders *reminders.Module
	push      Sender
}

// New returns the daily timer.
func New(opts Options) *Scheduler {
	return &Scheduler{clock: opts.Clock, pool: opts.Pool, settings: opts.Settings, reminders: opts.Reminders, push: opts.Push}
}

// LastPush is the result of the last daily push: its business-local day and how many
// subscriptions were tried and accepted it. It is stored as JSON in scheduler_state.
type LastPush struct {
	Date      string `json:"date"`
	Attempted int    `json:"attempted"`
	Delivered int    `json:"delivered"`
}

// State is what scheduler_state holds; a nil field has no stored value yet (or an unreadable one).
type State struct {
	Heartbeat       *time.Time
	PushCheckedDate *string
	LastPush        *LastPush
}

// Run calls Tick once at start and then on every value from ticks, until ctx is done. A failed
// tick logs code=scheduler_tick_failed and the next tick tries again.
func (s *Scheduler) Run(ctx context.Context, ticks <-chan time.Time) {
	s.tickOrLog(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			s.tickOrLog(ctx)
		}
	}
}

func (s *Scheduler) tickOrLog(ctx context.Context) {
	if err := s.Tick(ctx); err != nil {
		// Never log err: a database error can quote a stored value.
		slog.WarnContext(ctx, "daily timer tick failed", "code", "scheduler_tick_failed")
	}
}

// Tick is one minute of the timer (design doc item 58):
//  1. write the heartbeat;
//  2. before setup (no settings row), stop;
//  3. outside [push hour, 20:00) business-local time, or when today is already checked, stop;
//  4. count DueList in a read-only REPEATABLE READ transaction; a failure records nothing;
//  5. when the count is above 0, send one push;
//  6. in one write transaction, record today as checked and, when it sent, last_push.
//
// A send that fails before any attempt records nothing. A send that fails after some attempts
// (deleting a gone subscription failed) logs code=push_send_partial and still records the day,
// so the next tick does not send the morning push again.
func (s *Scheduler) Tick(ctx context.Context) error {
	now := s.clock.Now()
	if err := sqlc.New(s.pool).UpsertState(ctx, sqlc.UpsertStateParams{
		Name: keyHeartbeat, Value: now.UTC().Format(time.RFC3339), Now: now,
	}); err != nil {
		return fmt.Errorf("write heartbeat: %w", err)
	}
	biz, err := s.settings.Get(ctx, s.pool)
	if errors.As(err, &problem.NotFound{}) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("daily timer: %w", err)
	}
	tz, err := time.LoadLocation(biz.Timezone)
	if err != nil {
		return fmt.Errorf("daily timer: load the business time zone: %w", err)
	}
	loc, err := locale.For(biz.Locale)
	if err != nil {
		return fmt.Errorf("daily timer: %w", err)
	}
	local := now.In(tz)
	if local.Hour() < biz.PushHour || local.Hour() >= pushEndHour {
		return nil
	}
	today := local.Format(dateLayout)
	st, err := s.State(ctx, s.pool)
	if err != nil {
		return err
	}
	if st.PushCheckedDate != nil && *st.PushCheckedDate == today {
		return nil
	}
	var items []reminders.DueItem
	err = db.WithTx(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		if items, err = s.reminders.DueList(ctx, tx, now); err != nil {
			return fmt.Errorf("count the due list: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("daily timer: %w", err)
	}
	var last *LastPush
	if n := len(items); n > 0 {
		res, err := s.push.SendAll(ctx, s.pool, push.Payload{Title: biz.Name, Body: loc.PushBody(n), Count: n, URL: pushURL})
		if err != nil && res.Attempted == 0 {
			return fmt.Errorf("daily timer: send the push: %w", err)
		}
		if err != nil {
			slog.WarnContext(ctx, "daily push stopped part way", "code", "push_send_partial")
		}
		last = &LastPush{Date: today, Attempted: res.Attempted, Delivered: res.Delivered}
	}
	return s.record(ctx, today, last, now)
}

// record marks today as checked and stores last (when it sent) in one write transaction.
func (s *Scheduler) record(ctx context.Context, today string, last *LastPush, now time.Time) error {
	err := db.WithTx(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		queries := sqlc.New(tx)
		if err := queries.UpsertState(ctx, sqlc.UpsertStateParams{Name: keyPushCheckedDate, Value: today, Now: now}); err != nil {
			return fmt.Errorf("write push_checked_date: %w", err)
		}
		if last == nil {
			return nil
		}
		value, err := json.Marshal(last)
		if err != nil {
			return fmt.Errorf("encode last_push: %w", err)
		}
		if err := queries.UpsertState(ctx, sqlc.UpsertStateParams{Name: keyLastPush, Value: string(value), Now: now}); err != nil {
			return fmt.Errorf("write last_push: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("daily timer: %w", err)
	}
	return nil
}

// State reads the scheduler_state keys. A stored heartbeat or last_push it cannot parse is left
// nil and logs code=heartbeat_invalid or code=last_push_invalid, never the value.
func (s *Scheduler) State(ctx context.Context, q db.DBTX) (State, error) {
	rows, err := sqlc.New(q).ListState(ctx)
	if err != nil {
		return State{}, fmt.Errorf("list scheduler state: %w", err)
	}
	var st State
	for _, r := range rows {
		switch r.Name {
		case keyHeartbeat:
			t, err := time.Parse(time.RFC3339, r.Value)
			if err != nil {
				slog.WarnContext(ctx, "stored heartbeat is unreadable", "code", "heartbeat_invalid")
				continue
			}
			st.Heartbeat = &t
		case keyPushCheckedDate:
			st.PushCheckedDate = &r.Value
		case keyLastPush:
			var last LastPush
			if err := json.Unmarshal([]byte(r.Value), &last); err != nil {
				slog.WarnContext(ctx, "stored last push is unreadable", "code", "last_push_invalid")
				continue
			}
			st.LastPush = &last
		}
	}
	return st, nil
}
