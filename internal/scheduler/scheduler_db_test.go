//go:build db

package scheduler_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/push"
	"github.com/jnaranj09/open-customer/internal/reminders"
	"github.com/jnaranj09/open-customer/internal/scheduler"
	"github.com/jnaranj09/open-customer/internal/services"
	"github.com/jnaranj09/open-customer/internal/settings"
	"github.com/jnaranj09/open-customer/internal/templates"
)

const businessName = "Estudio Sol"

// fakeSender records every SendAll call and answers with res and err. Run calls it from its own
// goroutine, so it is locked.
type fakeSender struct {
	mu    sync.Mutex
	calls []push.Payload
	res   push.SendResult
	err   error
}

func (f *fakeSender) SendAll(_ context.Context, _ db.DBTX, p push.Payload) (push.SendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, p)
	return f.res, f.err
}

func (f *fakeSender) sent() []push.Payload {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]push.Payload(nil), f.calls...)
}

// env is a migrated database with the real modules, all on one fake clock that starts on
// 2026-09-30 10:00 in Europe/Madrid, so the rows made before a tick are in the past.
type env struct {
	pool      *pgxpool.Pool
	clock     *clock.Fake
	tz        *time.Location
	sender    *fakeSender
	s         *scheduler.Scheduler
	settings  *settings.Module
	reminders *reminders.Module
	services  *services.Module
	templates *templates.Module
	clients   *clients.Module
	appts     *appointments.Module
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := dbtest.Pool(t)
	tz, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatal(err)
	}
	fake := clock.NewFake(time.Date(2026, 9, 30, 10, 0, 0, 0, tz))
	attrs := attributes.New(attributes.Options{Clock: fake})
	svcs := services.New(services.Options{Clock: fake})
	e := env{
		pool: pool, clock: fake, tz: tz, sender: &fakeSender{res: push.SendResult{Attempted: 1, Delivered: 1}},
		settings:  settings.New(settings.Options{Clock: fake}),
		reminders: reminders.New(reminders.Options{Clock: fake, Attributes: attrs}),
		services:  svcs,
		templates: templates.New(templates.Options{Clock: fake, Attributes: attrs}),
		clients:   clients.New(clients.Options{Clock: fake, Attributes: attrs}),
		appts:     appointments.New(appointments.Options{Clock: fake, Services: svcs, Attributes: attrs}),
	}
	e.s = scheduler.New(scheduler.Options{Clock: fake, Pool: pool, Settings: e.settings, Reminders: e.reminders, Push: e.sender})
	return e
}

// newSetUpEnv is newEnv after setup: Europe/Madrid, push hour 8 (the default).
func newSetUpEnv(t *testing.T) env {
	t.Helper()
	e := newEnv(t)
	if _, err := e.settings.Create(t.Context(), e.pool, settings.SettingsInput{
		Name: businessName, Timezone: "Europe/Madrid", Locale: "es", Currency: "EUR", CountryCode: "ES",
		AccentColor: "#aa3366",
	}); err != nil {
		t.Fatalf("create settings: %v", err)
	}
	return e
}

// at is a business-local time.
func (e env) at(y int, m time.Month, d, h, minute int) time.Time {
	return time.Date(y, m, d, h, minute, 0, 0, e.tz)
}

func (e env) service(t *testing.T, name string, minutes int) uuid.UUID {
	t.Helper()
	s, err := e.services.Create(t.Context(), e.pool, services.ServiceInput{Name: name, DurationMin: minutes, Color: "#aa3366"})
	if err != nil {
		t.Fatalf("create service %s: %v", name, err)
	}
	return s.ID
}

func (e env) template(t *testing.T) uuid.UUID {
	t.Helper()
	tpl, err := e.templates.Create(t.Context(), e.pool, templates.TemplateInput{Name: "Cita", Body: "Hola {{client.first_name}}"})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	return tpl.ID
}

func (e env) client(t *testing.T, name string) uuid.UUID {
	t.Helper()
	c, err := e.clients.Create(t.Context(), e.pool, clients.ClientInput{Name: name, AllowDuplicatePhone: true}, "ES")
	if err != nil {
		t.Fatalf("create client %s: %v", name, err)
	}
	return c.ID
}

func (e env) book(t *testing.T, clientID uuid.UUID, starts time.Time, serviceIDs ...uuid.UUID) {
	t.Helper()
	if _, err := e.appts.Create(t.Context(), e.pool, appointments.AppointmentInput{
		ClientID: clientID, StartsAt: starts, ServiceIDs: serviceIDs, AllowOverlap: true,
	}); err != nil {
		t.Fatalf("book appointment: %v", err)
	}
}

func (e env) rule(t *testing.T, in reminders.RuleInput) {
	t.Helper()
	if _, err := e.reminders.CreateRule(t.Context(), e.pool, in); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
}

// beforeItem adds a "before" rule (one day ahead) and Ana Rojas's appointment on 2026-10-23
// 15:00: one message due from 2026-10-22 until the appointment starts.
func (e env) beforeItem(t *testing.T) {
	t.Helper()
	e.rule(t, reminders.RuleInput{Kind: reminders.KindBefore, OffsetDays: 1, TemplateID: e.template(t), IncludeOverdue: true})
	e.book(t, e.client(t, "Ana Rojas"), e.at(2026, 10, 23, 15, 0), e.service(t, "Pedicure", 30))
}

// tickAt sets the clock to now and runs one Tick, which must succeed.
func (e env) tickAt(t *testing.T, now time.Time) {
	t.Helper()
	e.clock.Set(now)
	if err := e.s.Tick(t.Context()); err != nil {
		t.Fatalf("Tick at %v: %v", now, err)
	}
}

func (e env) state(t *testing.T) scheduler.State {
	t.Helper()
	st, err := e.s.State(t.Context(), e.pool)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	return st
}

func (e env) wantCalls(t *testing.T, n int) []push.Payload {
	t.Helper()
	calls := e.sender.sent()
	if len(calls) != n {
		t.Fatalf("SendAll calls = %d (%+v), want %d", len(calls), calls, n)
	}
	return calls
}

func wantCheckedDate(t *testing.T, st scheduler.State, want string) {
	t.Helper()
	if st.PushCheckedDate == nil || *st.PushCheckedDate != want {
		t.Fatalf("PushCheckedDate = %v, want %q", st.PushCheckedDate, want)
	}
}

func wantNoCheckedDate(t *testing.T, st scheduler.State) {
	t.Helper()
	if st.PushCheckedDate != nil {
		t.Fatalf("PushCheckedDate = %q, want none", *st.PushCheckedDate)
	}
}

func wantLastPush(t *testing.T, st scheduler.State, want scheduler.LastPush) {
	t.Helper()
	if st.LastPush == nil || *st.LastPush != want {
		t.Fatalf("LastPush = %+v, want %+v", st.LastPush, want)
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	return &buf
}

func TestTickSendsPushOnce(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)

	e.tickAt(t, e.at(2026, 10, 22, 8, 0))
	e.tickAt(t, e.at(2026, 10, 22, 8, 1))

	calls := e.wantCalls(t, 1)
	want := push.Payload{Title: businessName, Body: "Tienes 1 mensaje para enviar hoy", Count: 1, URL: "/enviar"}
	if calls[0] != want {
		t.Fatalf("payload = %+v, want %+v", calls[0], want)
	}
	st := e.state(t)
	wantCheckedDate(t, st, "2026-10-22")
	wantLastPush(t, st, scheduler.LastPush{Date: "2026-10-22", Attempted: 1, Delivered: 1})
}

func TestTickBeforePushHourDoesNothing(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)

	e.tickAt(t, e.at(2026, 10, 22, 7, 59))

	e.wantCalls(t, 0)
	wantNoCheckedDate(t, e.state(t))
}

func TestTickLateStartBefore20Sends(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)

	e.tickAt(t, e.at(2026, 10, 22, 19, 59))

	e.wantCalls(t, 1)
}

func TestTickFrom20DoesNotSend(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)

	e.tickAt(t, e.at(2026, 10, 22, 20, 0))

	e.wantCalls(t, 0)
	wantNoCheckedDate(t, e.state(t))
}

func TestTickZeroCountRecordsDateWithoutPush(t *testing.T) {
	e := newSetUpEnv(t)

	e.tickAt(t, e.at(2026, 10, 22, 8, 0))

	e.wantCalls(t, 0)
	st := e.state(t)
	wantCheckedDate(t, st, "2026-10-22")
	if st.LastPush != nil {
		t.Fatalf("LastPush = %+v, want none: nothing was sent", *st.LastPush)
	}
}

// TestTickNextLocalDaySendsAgain: 23:00Z on 2026-10-22 is already 01:00 on 2026-10-23 in Madrid,
// before the push hour, so it sends nothing; 08:00 local that day sends the second push.
func TestTickNextLocalDaySendsAgain(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)
	e.tickAt(t, e.at(2026, 10, 22, 8, 0))

	e.tickAt(t, time.Date(2026, 10, 22, 23, 0, 0, 0, time.UTC))
	e.wantCalls(t, 1)

	e.tickAt(t, e.at(2026, 10, 23, 8, 0))
	e.wantCalls(t, 2)
	wantCheckedDate(t, e.state(t), "2026-10-23")
}

// TestTickCountsBookingPrompts: Marta Solano's come-back after her 2026-10-01 Acrílico falls due
// on 2026-10-22 while she has a later booking without the return service, so it is a booking
// prompt; with Ana Rojas's "before" message the push counts 2.
func TestTickCountsBookingPrompts(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)
	acrilico, relleno, manicura := e.service(t, "Acrílico", 60), e.service(t, "Relleno", 45), e.service(t, "Manicura", 30)
	e.rule(t, reminders.RuleInput{
		Kind: reminders.KindComeBack, ServiceID: uuid.NullUUID{UUID: acrilico, Valid: true},
		ReturnServiceID: uuid.NullUUID{UUID: relleno, Valid: true}, OffsetDays: 21, TemplateID: e.template(t),
	})
	marta := e.client(t, "Marta Solano")
	e.book(t, marta, e.at(2026, 10, 1, 15, 0), acrilico)
	e.book(t, marta, e.at(2026, 11, 12, 10, 0), manicura)

	e.tickAt(t, e.at(2026, 10, 22, 8, 0))

	calls := e.wantCalls(t, 1)
	if calls[0].Count != 2 || calls[0].Body != "Tienes 2 mensajes para enviar hoy" {
		t.Fatalf("payload = %+v, want count 2 with the plural text", calls[0])
	}
}

func TestTickBeforeSetupWritesOnlyHeartbeat(t *testing.T) {
	e := newEnv(t)
	now := e.at(2026, 10, 22, 8, 0)

	e.tickAt(t, now)

	e.wantCalls(t, 0)
	st := e.state(t)
	if st.Heartbeat == nil || !st.Heartbeat.Equal(now) {
		t.Fatalf("Heartbeat = %v, want %v", st.Heartbeat, now)
	}
	wantNoCheckedDate(t, st)
}

func TestTickCountFailureRecordsNothing(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)
	if _, err := e.pool.Exec(t.Context(), `ALTER TABLE reminder_rules RENAME TO reminder_rules_gone`); err != nil {
		t.Fatalf("rename reminder_rules: %v", err)
	}

	e.clock.Set(e.at(2026, 10, 22, 8, 0))
	if err := e.s.Tick(t.Context()); err == nil {
		t.Fatal("Tick = nil, want the count error")
	}

	e.wantCalls(t, 0)
	wantNoCheckedDate(t, e.state(t))
}

func TestTickRecordsAttemptedAndDelivered(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)
	e.sender.res = push.SendResult{Attempted: 2, Delivered: 1}

	e.tickAt(t, e.at(2026, 10, 22, 8, 0))

	wantLastPush(t, e.state(t), scheduler.LastPush{Date: "2026-10-22", Attempted: 2, Delivered: 1})
}

// TestTickPartialSendStillRecords: SendAll stopped after one attempt (a gone subscription could
// not be deleted). The push went out, so the day is recorded and the next tick does not re-send.
func TestTickPartialSendStillRecords(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)
	e.sender.res = push.SendResult{Attempted: 1, Delivered: 1}
	e.sender.err = errors.New("delete gone subscription: connection lost")
	logs := captureLog(t)

	e.tickAt(t, e.at(2026, 10, 22, 8, 0))
	e.tickAt(t, e.at(2026, 10, 22, 8, 1))

	e.wantCalls(t, 1)
	st := e.state(t)
	wantCheckedDate(t, st, "2026-10-22")
	wantLastPush(t, st, scheduler.LastPush{Date: "2026-10-22", Attempted: 1, Delivered: 1})
	if !strings.Contains(logs.String(), "code=push_send_partial") {
		t.Fatalf("log has no code=push_send_partial:\n%s", logs.String())
	}
}

func TestTickSendFailureRecordsNothing(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)
	e.sender.res = push.SendResult{}
	e.sender.err = errors.New("list subscriptions: connection lost")

	e.clock.Set(e.at(2026, 10, 22, 8, 0))
	if err := e.s.Tick(t.Context()); err == nil {
		t.Fatal("Tick = nil, want the send error")
	}

	st := e.state(t)
	wantNoCheckedDate(t, st)
	if st.LastPush != nil {
		t.Fatalf("LastPush = %+v, want none", *st.LastPush)
	}
}

func TestStateCorruptLastPushLogsCode(t *testing.T) {
	e := newEnv(t)
	if err := sqlc.New(e.pool).UpsertState(t.Context(), sqlc.UpsertStateParams{
		Name: "last_push", Value: "{not json", Now: e.clock.Now(),
	}); err != nil {
		t.Fatalf("store last_push: %v", err)
	}
	logs := captureLog(t)

	if st := e.state(t); st.LastPush != nil {
		t.Fatalf("LastPush = %+v, want none for a corrupt row", *st.LastPush)
	}
	if !strings.Contains(logs.String(), "code=last_push_invalid") {
		t.Fatalf("log has no code=last_push_invalid:\n%s", logs.String())
	}
}

func TestStateCorruptHeartbeatLogsCode(t *testing.T) {
	e := newEnv(t)
	if err := sqlc.New(e.pool).UpsertState(t.Context(), sqlc.UpsertStateParams{
		Name: "heartbeat", Value: "yesterday", Now: e.clock.Now(),
	}); err != nil {
		t.Fatalf("store heartbeat: %v", err)
	}
	logs := captureLog(t)

	if st := e.state(t); st.Heartbeat != nil {
		t.Fatalf("Heartbeat = %v, want none for a corrupt row", *st.Heartbeat)
	}
	if !strings.Contains(logs.String(), "code=heartbeat_invalid") {
		t.Fatalf("log has no code=heartbeat_invalid:\n%s", logs.String())
	}
}

// countingClock counts the Now calls; Tick reads the clock once, first thing.
type countingClock struct {
	*clock.Fake
	mu sync.Mutex
	n  int
}

func (c *countingClock) Now() time.Time {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return c.Fake.Now()
}

func (c *countingClock) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// TestRunTicksUntilCancelled sends two ticks on an unbuffered channel, so each send returns only
// once Run has taken it, then cancels: Run returns after three ticks (one at start, one per tick)
// and the heartbeat was written.
func TestRunTicksUntilCancelled(t *testing.T) {
	e := newEnv(t)
	clk := &countingClock{Fake: e.clock}
	s := scheduler.New(scheduler.Options{Clock: clk, Pool: e.pool, Settings: e.settings, Reminders: e.reminders, Push: e.sender})
	ctx, cancel := context.WithCancel(t.Context())
	ticks := make(chan time.Time)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx, ticks)
	}()

	ticks <- e.clock.Now()
	ticks <- e.clock.Now()
	cancel()
	<-done

	if n := clk.count(); n != 3 {
		t.Fatalf("Tick ran %d times, want 3", n)
	}
	if st := e.state(t); st.Heartbeat == nil || !st.Heartbeat.Equal(e.clock.Now()) {
		t.Fatalf("Heartbeat = %v, want %v", st.Heartbeat, e.clock.Now())
	}
}

// TestTickCountsInASnapshot: the due list is counted in a read-only REPEATABLE READ transaction,
// so its queries see one snapshot.
func TestTickCountsInASnapshot(t *testing.T) {
	e := newSetUpEnv(t)
	e.beforeItem(t)
	rec := &dbtest.TxModeRecorder{Pool: e.pool}
	scheduler.SetBeginner(e.s, rec)

	e.tickAt(t, e.at(2026, 10, 22, 8, 0))

	e.wantCalls(t, 1)
	if got := rec.Modes(); len(got) == 0 || got[0] != dbtest.SnapshotMode {
		t.Fatalf("transaction modes = %+v, want the count first in %+v", got, dbtest.SnapshotMode)
	}
}
