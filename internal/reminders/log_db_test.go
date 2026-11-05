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

// logSetup is a "before" rule and a come-back rule, and Ana Rojas's visit with both.
type logSetup struct {
	e        dueEnv
	ana      uuid.UUID
	before   Rule
	comeBack Rule
	visit    time.Time
	apptID   uuid.UUID
}

func newLogSetup(t *testing.T) logSetup {
	t.Helper()
	e := newDueEnv(t)
	acrilico := e.service(t, "Acrílico", 60, nil)
	ana := e.client(t, "Ana Rojas", true)
	a := e.book(t, ana.ID, e.at(2026, 10, 23, 15, 0), acrilico)
	tpl := e.template(t, "Cita", "Hola {{client.first_name}}")
	return logSetup{
		e: e, ana: ana.ID, visit: a.StartsAt, apptID: a.ID,
		before:   e.rule(t, RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: tpl}),
		comeBack: e.rule(t, RuleInput{Kind: KindComeBack, ServiceID: nu(acrilico), OffsetDays: 21, TemplateID: tpl}),
	}
}

func (s logSetup) beforeEntry(action Action) LogEntry {
	body, digits := "Hola Ana", "34612345678"
	return LogEntry{
		RuleID: s.before.ID, AppointmentID: s.apptID, AnchorAt: &s.visit, Action: action,
		Body: &body, PhoneDigits: &digits, ActedAt: s.e.now(),
	}
}

func (s logSetup) comeBackEntry() LogEntry {
	return LogEntry{RuleID: s.comeBack.ID, AppointmentID: s.apptID, Action: ActionSkipped, ActedAt: s.e.now()}
}

func TestRecordSentCreates(t *testing.T) {
	s := newLogSetup(t)
	want := s.beforeEntry(ActionSent)

	it, created, err := s.e.m.Record(t.Context(), s.e.pool, want)
	if err != nil || !created {
		t.Fatalf("Record = %v, %v; want created", created, err)
	}
	if it.Kind != KindBefore || it.ClientID != s.ana || it.ClientName != "Ana Rojas" || it.ID == uuid.Nil {
		t.Fatalf("item = %+v; want kind before, Ana Rojas and an id", it)
	}
	if it.RuleID != want.RuleID || it.AppointmentID != want.AppointmentID || it.Action != ActionSent ||
		*it.Body != "Hola Ana" || *it.PhoneDigits != "34612345678" || !it.ActedAt.Equal(want.ActedAt) ||
		it.AnchorAt == nil || !it.AnchorAt.Equal(s.visit) {
		t.Fatalf("item = %+v; want the entry back", it)
	}
}

func TestRecordTwiceKeepsOneRow(t *testing.T) {
	tests := map[string]struct {
		entry  func(s logSetup) LogEntry
		first  Action
		second Action
	}{
		"before":    {func(s logSetup) LogEntry { return s.beforeEntry(ActionSent) }, ActionSent, ActionSkipped},
		"come back": {func(s logSetup) LogEntry { return s.comeBackEntry() }, ActionSkipped, ActionSent},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newLogSetup(t)
			firstEntry := tc.entry(s)
			firstEntry.Action = tc.first
			first, _, err := s.e.m.Record(t.Context(), s.e.pool, firstEntry)
			if err != nil {
				t.Fatalf("first Record: %v", err)
			}
			again := tc.entry(s)
			again.Action = tc.second

			second, created, err := s.e.m.Record(t.Context(), s.e.pool, again)
			if err != nil || created {
				t.Fatalf("second Record = %v, %v; want not created", created, err)
			}
			if second.ID != first.ID || second.Action != tc.first {
				t.Fatalf("second = %+v; want the first row %+v", second, first)
			}
			var n int
			if err := s.e.pool.QueryRow(t.Context(), `SELECT count(*) FROM reminder_log`).Scan(&n); err != nil || n != 1 {
				t.Fatalf("count = %d, %v; want 1", n, err)
			}
		})
	}
}

func TestRecordRejects(t *testing.T) {
	bad, sixteen, empty := "88-88", "1234567890123456", ""
	tests := map[string]struct {
		change func(s logSetup, e *LogEntry)
		field  string
	}{
		"unknown rule":          {func(_ logSetup, e *LogEntry) { e.RuleID = uuid.New() }, "rule_id"},
		"unknown appointment":   {func(_ logSetup, e *LogEntry) { e.AppointmentID = uuid.New() }, "appointment_id"},
		"before without anchor": {func(_ logSetup, e *LogEntry) { e.AnchorAt = nil }, "anchor_at"},
		"come back with anchor": {func(s logSetup, e *LogEntry) { e.RuleID, e.AnchorAt = s.comeBack.ID, &s.visit }, "anchor_at"},
		"bad phone digits":      {func(_ logSetup, e *LogEntry) { e.PhoneDigits = &bad }, "phone_digits"},
		"16 phone digits":       {func(_ logSetup, e *LogEntry) { e.PhoneDigits = &sixteen }, "phone_digits"},
		"empty phone digits":    {func(_ logSetup, e *LogEntry) { e.PhoneDigits = &empty }, "phone_digits"},
		"unknown action":        {func(_ logSetup, e *LogEntry) { e.Action = "later" }, "action"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newLogSetup(t)
			entry := s.beforeEntry(ActionSent)
			tc.change(s, &entry)

			_, created, err := s.e.m.Record(t.Context(), s.e.pool, entry)
			var v problem.Validation
			if !errors.As(err, &v) || created || len(v.Errors) != 1 || v.Errors[0].Field != tc.field {
				t.Fatalf("Record = %v, %v; want a 422 on %s", created, err, tc.field)
			}
		})
	}
}

// TestRecordAccepts15PhoneDigits: 15 digits is the longest international number (E.164).
func TestRecordAccepts15PhoneDigits(t *testing.T) {
	s := newLogSetup(t)
	entry := s.beforeEntry(ActionSent)
	fifteen := "123456789012345"
	entry.PhoneDigits = &fifteen

	it, created, err := s.e.m.Record(t.Context(), s.e.pool, entry)

	if err != nil || !created || it.PhoneDigits == nil || *it.PhoneDigits != fifteen {
		t.Fatalf("Record = %+v, %v, %v; want the 15 digits stored", it, created, err)
	}
}

func TestRecordSkippedWithoutPhone(t *testing.T) {
	s := newLogSetup(t)
	entry := s.beforeEntry(ActionSkipped)
	entry.Body, entry.PhoneDigits = nil, nil

	it, created, err := s.e.m.Record(t.Context(), s.e.pool, entry)
	if err != nil || !created {
		t.Fatalf("Record = %v, %v; want created", created, err)
	}
	if it.Body != nil || it.PhoneDigits != nil || it.Action != ActionSkipped {
		t.Fatalf("item = %+v; want a skip with no body and no phone", it)
	}
}

func TestRecordOnArchivedRule(t *testing.T) {
	s := newLogSetup(t)
	archived := true
	if _, err := s.e.m.UpdateRule(t.Context(), s.e.pool, s.before.ID, RulePatch{Archived: &archived}); err != nil {
		t.Fatalf("archive rule: %v", err)
	}

	_, created, err := s.e.m.Record(t.Context(), s.e.pool, s.beforeEntry(ActionSent))
	if err != nil || !created {
		t.Fatalf("Record = %v, %v; want created on an archived rule", created, err)
	}
}

func TestLogForClientNewestFirst(t *testing.T) {
	s := newLogSetup(t)
	older, newer := s.beforeEntry(ActionSent), s.comeBackEntry()
	older.ActedAt, newer.ActedAt = s.e.now().Add(-time.Hour), s.e.now()
	other := s.e.client(t, "Marta Solano", false)
	otherAppt := s.e.book(t, other.ID, s.e.at(2026, 10, 24, 10, 0), s.e.service(t, "Pedicure", 30, nil))
	for _, entry := range []LogEntry{older, newer, {
		RuleID: s.before.ID, AppointmentID: otherAppt.ID, AnchorAt: &otherAppt.StartsAt, Action: ActionSent, ActedAt: s.e.now(),
	}} {
		s.e.record(t, entry)
	}

	items, err := s.e.m.LogForClient(t.Context(), s.e.pool, s.ana)
	if err != nil {
		t.Fatalf("LogForClient: %v", err)
	}
	got := []Kind{}
	for _, it := range items {
		got = append(got, it.Kind)
	}
	if !slices.Equal(got, []Kind{KindComeBack, KindBefore}) {
		t.Fatalf("kinds = %v; want Ana's two rows, newest first, and none of Marta's", got)
	}
}

func TestLogForClientUnknownIsNotFound(t *testing.T) {
	s := newLogSetup(t)

	_, err := s.e.m.LogForClient(t.Context(), s.e.pool, uuid.New())
	if !errors.As(err, &problem.NotFound{}) {
		t.Fatalf("err = %v; want problem.NotFound", err)
	}
}

func TestLogForClientWithoutRowsIsEmpty(t *testing.T) {
	s := newLogSetup(t)

	items, err := s.e.m.LogForClient(t.Context(), s.e.pool, s.ana)
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("LogForClient = %v, %v; want an empty, non-nil slice", items, err)
	}
}

func TestLogForDateUsesLocalDay(t *testing.T) {
	s := newLogSetup(t)
	entry := s.beforeEntry(ActionSent)
	entry.ActedAt = time.Date(2026, 10, 24, 22, 30, 0, 0, time.UTC) // 2026-10-25 00:30 in Madrid
	s.e.record(t, entry)
	day := func(d int) time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, s.e.tz) }

	onNext, err := s.e.m.LogForDate(t.Context(), s.e.pool, day(25), s.e.tz)
	if err != nil {
		t.Fatalf("LogForDate 25: %v", err)
	}
	onPrev, err := s.e.m.LogForDate(t.Context(), s.e.pool, day(24), s.e.tz)
	if err != nil {
		t.Fatalf("LogForDate 24: %v", err)
	}
	if len(onNext) != 1 || onNext[0].ClientName != "Ana Rojas" || len(onPrev) != 0 {
		t.Fatalf("on the 25th = %+v, on the 24th = %+v; want the row only on the 25th", onNext, onPrev)
	}
}

// TestLogDBErrors fails each query of Record (a create, then the conflict path), LogForClient and
// LogForDate in turn. A case's prior entries are logged before the sweep.
func nothing(logSetup) []LogEntry { return nil }

func beforeLogged(s logSetup) []LogEntry { return []LogEntry{s.beforeEntry(ActionSent)} }

func TestLogDBErrors(t *testing.T) {
	tests := map[string]struct {
		prior func(s logSetup) []LogEntry
		op    func(t *testing.T, s logSetup, q db.DBTX) error
	}{
		"Record": {nothing, func(t *testing.T, s logSetup, q db.DBTX) error {
			t.Helper()
			_, _, err := s.e.m.Record(t.Context(), q, s.beforeEntry(ActionSent))
			return err
		}},
		"Record conflict": {beforeLogged, func(t *testing.T, s logSetup, q db.DBTX) error {
			t.Helper()
			_, _, err := s.e.m.Record(t.Context(), q, s.beforeEntry(ActionSent))
			return err
		}},
		"LogForClient": {beforeLogged, func(t *testing.T, s logSetup, q db.DBTX) error {
			t.Helper()
			_, err := s.e.m.LogForClient(t.Context(), q, s.ana)
			return err
		}},
		"LogForDate": {beforeLogged, func(t *testing.T, s logSetup, q db.DBTX) error {
			t.Helper()
			_, err := s.e.m.LogForDate(t.Context(), q, s.e.now(), s.e.tz)
			return err
		}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sweepDBErrors(t, func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
				t.Helper()
				s := newLogSetup(t)
				for _, entry := range tc.prior(s) {
					s.e.record(t, entry)
				}
				return s.e.pool, func(q db.DBTX) error { return tc.op(t, s, q) }
			})
		})
	}
}
