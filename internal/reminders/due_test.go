package reminders

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/appointments"
)

// The made-up services the due-list tests book.
var (
	acrilico = service{uuid.MustParse("019a0000-0000-7000-8000-000000000001"), "Acrílico"}
	relleno  = service{uuid.MustParse("019a0000-0000-7000-8000-000000000002"), "Relleno"}
	pedicure = service{uuid.MustParse("019a0000-0000-7000-8000-000000000003"), "Pedicure"}
)

const anaPhone = "34612345678"

type service struct {
	ID   uuid.UUID
	Name string
}

// dueBuilder builds the dueData of one due-list test, in Europe/Madrid unless a test sets TZ.
type dueBuilder struct {
	t *testing.T
	d dueData
}

func newDue(t *testing.T) *dueBuilder {
	t.Helper()
	return &dueBuilder{t: t, d: dueData{
		TZ: testZone(t), Locale: testLocale(t), Business: "Estudio Sol", Logged: map[logKey]bool{},
	}}
}

// at is a business-local time in the builder's zone.
func (b *dueBuilder) at(y int, m time.Month, d, h, minute int) time.Time {
	return time.Date(y, m, d, h, minute, 0, 0, b.d.TZ)
}

// beforeRule adds a "before" rule with offset days and returns it.
func (b *dueBuilder) beforeRule(offset int) ruleRow {
	r := ruleRow{
		ID: uuid.New(), Kind: KindBefore, OffsetDays: offset,
		TemplateBody: "Hola {{client.first_name}}, tu cita es el {{appointment.date}} a las {{appointment.time}} ({{appointment.services}})",
	}
	b.d.Rules = append(b.d.Rules, r)
	return r
}

// comeBackRule adds a come_back rule on svc with the effective return service ret.
func (b *dueBuilder) comeBackRule(svc, ret service, offset int) ruleRow {
	r := ruleRow{
		ID: uuid.New(), Kind: KindComeBack, ServiceID: uuid.NullUUID{UUID: svc.ID, Valid: true},
		ReturnServiceID: ret.ID, ReturnServiceName: ret.Name, OffsetDays: offset,
		TemplateBody: "Hola {{client.first_name}}, ¿reservamos tu {{appointment.services}}?",
	}
	b.d.Rules = append(b.d.Rules, r)
	return r
}

// appt is a scheduled appointment of Ana Rojas (with a phone) from starts to ends, created at
// created, with the given services.
func appt(starts, ends, created time.Time, svcs ...service) apptRow {
	phone := anaPhone
	a := apptRow{
		ID: uuid.New(), ClientID: anaID, ClientName: "Ana Rojas", PhoneDigits: &phone,
		StartsAt: starts, EndsAt: ends, CreatedAt: created, Status: appointments.Scheduled,
	}
	for _, s := range svcs {
		a.ServiceIDs = append(a.ServiceIDs, s.ID)
		a.ServiceNames = append(a.ServiceNames, s.Name)
	}
	return a
}

var anaID = uuid.MustParse("019a0000-0000-7000-8000-0000000000a1")

func (b *dueBuilder) before(a apptRow) apptRow {
	b.d.Before = append(b.d.Before, a)
	return a
}

func (b *dueBuilder) comeBack(a apptRow) apptRow {
	b.d.ComeBack = append(b.d.ComeBack, a)
	return a
}

// later adds a later appointment of a's client starting at starts with the given services.
func (b *dueBuilder) later(a apptRow, starts time.Time, svcs ...service) laterAppt {
	l := laterAppt{ID: uuid.New(), ClientID: a.ClientID, StartsAt: starts}
	for _, s := range svcs {
		l.ServiceIDs = append(l.ServiceIDs, s.ID)
		l.ServiceNames = append(l.ServiceNames, s.Name)
	}
	b.d.Later = append(b.d.Later, l)
	return l
}

func (b *dueBuilder) logBefore(r ruleRow, a apptRow) {
	b.d.Logged[logKey{RuleID: r.ID, AppointmentID: a.ID, AnchorAt: a.StartsAt.UTC()}] = true
}

func (b *dueBuilder) logComeBack(r ruleRow, a apptRow) {
	b.d.Logged[logKey{RuleID: r.ID, AppointmentID: a.ID}] = true
}

// run computes the list at now.
func (b *dueBuilder) run(now time.Time) []DueItem {
	return compute(b.d, now)
}

// one fails unless items holds exactly one item, and returns it.
func one(t *testing.T, items []DueItem) DueItem {
	t.Helper()
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1: %+v", len(items), items)
	}
	return items[0]
}

func none(t *testing.T, items []DueItem) {
	t.Helper()
	if len(items) != 0 {
		t.Fatalf("got %d items, want none: %+v", len(items), items)
	}
}

func day(t *testing.T, it DueItem, want time.Time) {
	t.Helper()
	if !it.DueDate.Equal(want) {
		t.Fatalf("DueDate = %v, want %v", it.DueDate, want)
	}
}

func TestBeforeNotSkippedByOwnAppointment(t *testing.T) {
	b := newDue(t)
	b.beforeRule(1)
	b.before(appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), acrilico))

	it := one(t, b.run(b.at(2026, 10, 22, 8, 0)))
	if it.Mode != ModeMessage || it.Late || it.Prompt != nil {
		t.Fatalf("mode %q late %v prompt %v, want message, not late, no prompt", it.Mode, it.Late, it.Prompt)
	}
	day(t, it, b.at(2026, 10, 22, 0, 0))
}

func TestBeforeHiddenAtStart(t *testing.T) {
	b := newDue(t)
	b.beforeRule(0)
	a := b.before(appt(b.at(2026, 10, 22, 15, 0), b.at(2026, 10, 22, 16, 0), b.at(2026, 10, 19, 10, 0), acrilico))

	t.Run("one second before", func(t *testing.T) {
		one(t, b.run(a.StartsAt.Add(-time.Second)))
	})
	t.Run("at the start", func(t *testing.T) {
		none(t, b.run(a.StartsAt))
	})
}

func TestBeforeOneItemForTwoServices(t *testing.T) {
	b := newDue(t)
	b.beforeRule(1)
	b.before(appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 17, 0), b.at(2026, 10, 19, 10, 0), acrilico, pedicure))

	it := one(t, b.run(b.at(2026, 10, 22, 8, 0)))
	if !strings.Contains(it.Text, "(Acrílico y Pedicure)") {
		t.Fatalf("text %q does not list both services", it.Text)
	}
}

func TestRescheduleGivesBeforeNewIdentity(t *testing.T) {
	b := newDue(t)
	r := b.beforeRule(1)
	old := appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), acrilico)
	b.logBefore(r, old)
	moved := old
	moved.StartsAt, moved.EndsAt = b.at(2026, 10, 23, 17, 0), b.at(2026, 10, 23, 18, 0)
	b.before(moved)

	it := one(t, b.run(b.at(2026, 10, 22, 8, 0)))
	if want := ":" + strconv.FormatInt(moved.StartsAt.Unix(), 10); !strings.HasSuffix(it.ID, want) {
		t.Fatalf("ID %q does not end in %q", it.ID, want)
	}
	if !strings.Contains(it.Text, "5:00 p. m.") {
		t.Fatalf("text %q does not show the new time", it.Text)
	}
}

func TestSameDayBookingNoOneDayBeforeButZeroDay(t *testing.T) {
	b := newDue(t)
	b.beforeRule(1)
	zero := b.beforeRule(0)
	b.before(appt(b.at(2026, 10, 22, 15, 0), b.at(2026, 10, 22, 16, 0), b.at(2026, 10, 22, 8, 30), acrilico))

	it := one(t, b.run(b.at(2026, 10, 22, 9, 0)))
	if it.RuleID != zero.ID {
		t.Fatalf("item of rule %s, want the 0-day rule %s", it.RuleID, zero.ID)
	}
}

func TestZeroDayBeforeEarlyAppointmentNotAtPushHour(t *testing.T) {
	b := newDue(t)
	b.beforeRule(0)
	b.before(appt(b.at(2026, 10, 22, 7, 0), b.at(2026, 10, 22, 8, 0), b.at(2026, 10, 19, 10, 0), acrilico))

	none(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestBeforeCancelledAndNoShowGiveNothing(t *testing.T) {
	for _, st := range []appointments.Status{appointments.Cancelled, appointments.NoShow} {
		t.Run(string(st), func(t *testing.T) {
			b := newDue(t)
			b.beforeRule(1)
			a := appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), acrilico)
			a.Status = st
			b.before(a)

			none(t, b.run(b.at(2026, 10, 22, 8, 0)))
		})
	}
}

func TestArchivedTemplateServiceOrReturnServiceGiveNoItems(t *testing.T) {
	tests := []struct {
		name    string
		archive func(*ruleRow)
	}{
		{"template", func(r *ruleRow) { r.TemplateArchived = true }},
		{"service", func(r *ruleRow) { r.ServiceArchived = true }},
		{"return service", func(r *ruleRow) { r.ReturnArchived = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := newDue(t)
			b.comeBackRule(acrilico, relleno, 21)
			tc.archive(&b.d.Rules[0])
			b.comeBack(appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 16, 0), b.at(2026, 9, 28, 10, 0), acrilico))

			none(t, b.run(b.at(2026, 10, 22, 8, 0)))
		})
	}
}

func TestArchivedTemplateGivesNoBeforeItems(t *testing.T) {
	b := newDue(t)
	b.beforeRule(1)
	b.d.Rules[0].TemplateArchived = true
	b.before(appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), acrilico))

	none(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

// lateVisit adds a done Acrílico visit created long ago whose 21-day come-back fell due on
// 2026-10-12, ten days before the tests' now of 2026-10-22.
func lateVisit(b *dueBuilder) apptRow {
	return b.comeBack(appt(b.at(2026, 9, 21, 15, 0), b.at(2026, 9, 21, 16, 0), b.at(2026, 9, 1, 10, 0), acrilico))
}

func TestAppliesFromTodayOnward(t *testing.T) {
	now := time.Date(2026, 10, 22, 8, 0, 0, 0, testZone(t))

	t.Run("hides late come-backs", func(t *testing.T) {
		b := newDue(t)
		b.comeBackRule(acrilico, relleno, 21)
		b.d.Rules[0].AppliesFrom = &now
		lateVisit(b)
		none(t, b.run(now))
	})
	t.Run("keeps a come-back whose due day is ahead", func(t *testing.T) {
		b := newDue(t)
		b.comeBackRule(acrilico, relleno, 21)
		b.d.Rules[0].AppliesFrom = &now
		b.comeBack(appt(b.at(2026, 10, 15, 15, 0), b.at(2026, 10, 15, 16, 0), b.at(2026, 10, 1, 10, 0), acrilico))
		none(t, b.run(now))
		day(t, one(t, b.run(now.AddDate(0, 0, 14))), b.at(2026, 11, 5, 0, 0))
	})
	t.Run("null lists late come-backs", func(t *testing.T) {
		b := newDue(t)
		b.comeBackRule(acrilico, relleno, 21)
		lateVisit(b)
		if it := one(t, b.run(now)); !it.Late {
			t.Fatalf("item not late: %+v", it)
		}
	})
	t.Run("lowered offset gives no late item", func(t *testing.T) {
		b := newDue(t)
		b.comeBackRule(acrilico, relleno, 21)
		b.d.Rules[0].OffsetDays, b.d.Rules[0].AppliesFrom = 14, &now
		b.comeBack(appt(b.at(2026, 10, 5, 15, 0), b.at(2026, 10, 5, 16, 0), b.at(2026, 9, 1, 10, 0), acrilico))
		none(t, b.run(now))
	})
}

func TestNewRuleTodayGivesTodaysItem(t *testing.T) {
	b := newDue(t)
	now := b.at(2026, 10, 22, 8, 0)
	b.comeBackRule(acrilico, relleno, 21)
	b.d.Rules[0].AppliesFrom = &now
	b.comeBack(appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 16, 0), b.at(2026, 9, 28, 10, 0), acrilico))

	if it := one(t, b.run(now)); it.Late {
		t.Fatalf("today's item is late: %+v", it)
	}
}

func TestHandledItemsExcluded(t *testing.T) {
	b := newDue(t)
	before := b.beforeRule(1)
	cb := b.comeBackRule(acrilico, relleno, 21)
	tomorrow := b.before(appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), pedicure))
	visit := b.comeBack(appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 16, 0), b.at(2026, 9, 28, 10, 0), acrilico))
	b.logBefore(before, tomorrow)
	b.logComeBack(cb, visit)

	none(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestLookBack60Days(t *testing.T) {
	// The visit's 21-day come-back falls due on 2026-08-22.
	build := func(t *testing.T) *dueBuilder {
		t.Helper()
		b := newDue(t)
		b.comeBackRule(acrilico, relleno, 21)
		b.comeBack(appt(b.at(2026, 8, 1, 15, 0), b.at(2026, 8, 1, 16, 0), b.at(2026, 7, 1, 10, 0), acrilico))
		return b
	}
	t.Run("60 days past the due day it is listed and late", func(t *testing.T) {
		b := build(t)
		if it := one(t, b.run(b.at(2026, 10, 21, 8, 0))); !it.Late {
			t.Fatalf("item not late: %+v", it)
		}
	})
	t.Run("61 days past the due day it is gone", func(t *testing.T) {
		b := build(t)
		none(t, b.run(b.at(2026, 10, 22, 8, 0)))
	})
}

func TestDowntimeCatchUp(t *testing.T) {
	b := newDue(t)
	b.beforeRule(1)
	b.comeBackRule(acrilico, relleno, 21)
	b.before(appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), pedicure))
	b.comeBack(appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 16, 0), b.at(2026, 9, 28, 10, 0), acrilico))

	it := one(t, b.run(b.at(2026, 10, 27, 8, 0)))
	if it.Kind != KindComeBack || !it.Late {
		t.Fatalf("kind %q late %v, want a late come-back", it.Kind, it.Late)
	}
}

func TestDueDayAtLocalMidnightAcrossDST(t *testing.T) {
	t.Run("come-back counts from local midnight", func(t *testing.T) {
		b := newDue(t)
		b.comeBackRule(acrilico, relleno, 21)
		b.comeBack(appt(b.at(2026, 10, 4, 15, 0), b.at(2026, 10, 4, 16, 0), b.at(2026, 9, 28, 10, 0), acrilico))
		none(t, b.run(time.Date(2026, 10, 24, 21, 59, 0, 0, time.UTC)))
		day(t, one(t, b.run(time.Date(2026, 10, 24, 22, 1, 0, 0, time.UTC))), b.at(2026, 10, 25, 0, 0))
	})
	t.Run("before is due the local day before", func(t *testing.T) {
		b := newDue(t)
		b.beforeRule(1)
		b.before(appt(b.at(2026, 10, 26, 0, 30), b.at(2026, 10, 26, 1, 30), b.at(2026, 10, 19, 10, 0), acrilico))
		day(t, one(t, b.run(b.at(2026, 10, 25, 8, 0))), b.at(2026, 10, 25, 0, 0))
	})
}

func TestSortedByDueDayThenClientName(t *testing.T) {
	b := newDue(t)
	b.beforeRule(1)
	b.comeBackRule(acrilico, relleno, 21)
	marta := appt(b.at(2026, 10, 23, 10, 0), b.at(2026, 10, 23, 11, 0), b.at(2026, 10, 19, 10, 0), pedicure)
	marta.ClientID, marta.ClientName = uuid.New(), "Marta Solano"
	b.before(marta)
	b.before(appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), pedicure))
	old := appt(b.at(2026, 9, 25, 15, 0), b.at(2026, 9, 25, 16, 0), b.at(2026, 9, 1, 10, 0), acrilico)
	old.ClientID, old.ClientName = uuid.New(), "Lucía Rojas"
	b.comeBack(old)

	items := b.run(b.at(2026, 10, 22, 8, 0))
	var got []string
	for _, it := range items {
		got = append(got, it.ClientName)
	}
	if want := []string{"Lucía Rojas", "Ana Rojas", "Marta Solano"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order %v, want %v", got, want)
	}
}

func TestSortedByIDOnSameDayAndClient(t *testing.T) {
	b := newDue(t)
	b.beforeRule(1)
	b.beforeRule(1)
	// The rule listed first gets the larger ID, so the input order is not the answer.
	b.d.Rules[0].ID = uuid.MustParse("019a0000-0000-7000-8000-0000000000f2")
	b.d.Rules[1].ID = uuid.MustParse("019a0000-0000-7000-8000-0000000000f1")
	b.before(appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), pedicure))

	items := b.run(b.at(2026, 10, 22, 8, 0))
	if len(items) != 2 || items[0].RuleID != b.d.Rules[1].ID {
		t.Fatalf("items not ordered by ID: %+v", items)
	}
}

func TestComputeClientWithoutPhone(t *testing.T) {
	b := newDue(t)
	b.beforeRule(1)
	a := appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), pedicure)
	a.PhoneDigits = nil
	b.before(a)

	it := one(t, b.run(b.at(2026, 10, 22, 8, 0)))
	if it.PhoneDigits != nil || it.WaLink != nil || it.Text == "" {
		t.Fatalf("phone %v link %v text %q, want nil, nil and a text", it.PhoneDigits, it.WaLink, it.Text)
	}
}

func TestDueItemIDFormat(t *testing.T) {
	t.Run("before", func(t *testing.T) {
		b := newDue(t)
		r := b.beforeRule(1)
		a := b.before(appt(b.at(2026, 10, 23, 15, 0), b.at(2026, 10, 23, 16, 0), b.at(2026, 10, 19, 10, 0), pedicure))
		it := one(t, b.run(b.at(2026, 10, 22, 8, 0)))
		if want := r.ID.String() + ":" + a.ID.String() + ":" + strconv.FormatInt(a.StartsAt.Unix(), 10); it.ID != want {
			t.Fatalf("ID = %q, want %q", it.ID, want)
		}
		if it.AnchorAt == nil || !it.AnchorAt.Equal(a.StartsAt) {
			t.Fatalf("AnchorAt = %v, want %v", it.AnchorAt, a.StartsAt)
		}
	})
	t.Run("come-back", func(t *testing.T) {
		b := newDue(t)
		r := b.comeBackRule(acrilico, relleno, 21)
		a := b.comeBack(appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 16, 0), b.at(2026, 9, 28, 10, 0), acrilico))
		it := one(t, b.run(b.at(2026, 10, 22, 8, 0)))
		if want := r.ID.String() + ":" + a.ID.String(); it.ID != want || it.AnchorAt != nil {
			t.Fatalf("ID = %q AnchorAt %v, want %q and nil", it.ID, it.AnchorAt, want)
		}
	})
}
