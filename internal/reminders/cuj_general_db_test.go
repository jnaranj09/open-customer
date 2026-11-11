//go:build db

package reminders

import (
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/appointments"
)

// General rules end to end, through the loader and the real modules (reminders-and-replies design,
// CUJ R1-R4; item 28). The clock starts on 2026-09-30, so each 1 Oct visit is booked ahead.

func (e dueEnv) dueAt(t *testing.T, now time.Time) []DueItem {
	t.Helper()
	items, err := e.m.DueList(t.Context(), e.pool, now)
	if err != nil {
		t.Fatalf("DueList: %v", err)
	}
	return items
}

func (e dueEnv) general(t *testing.T, days int) Rule {
	t.Helper()
	return e.rule(t, RuleInput{
		Kind: KindComeBack, OffsetDays: days, IncludeOverdue: true,
		TemplateID: e.template(t, "Volver", "Hola {{client.first_name}}, ¿te agendo?"),
	})
}

func TestCUJR1OneGeneralRuleCoversEveryone(t *testing.T) {
	e := newDueEnv(t)
	acrilico := e.service(t, "Acrílico", 60, nil)
	g := e.general(t, 30)
	ana := e.client(t, "Ana Rojas", true)
	visit := e.book(t, ana.ID, e.at(2026, 10, 1, 10, 0), acrilico)

	it := one(t, e.dueAt(t, e.at(2026, 10, 31, 9, 0)))
	if it.RuleID != g.ID || it.AppointmentID != visit.ID || !it.General || it.OffsetDays != 30 ||
		len(it.SourceServices) != 1 || it.SourceServices[0] != "Acrílico" {
		t.Fatalf("item = %+v, want Ana's general 30-day item for the Acrílico visit", it)
	}

	e.clock.Set(e.at(2026, 10, 31, 9, 0))
	next := e.book(t, ana.ID, e.at(2026, 11, 20, 10, 0), acrilico)
	none(t, e.dueAt(t, e.at(2026, 10, 31, 9, 30)))

	if _, err := e.appts.SetStatus(t.Context(), e.pool, next.ID, appointments.Cancelled, true); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	one(t, e.dueAt(t, e.at(2026, 10, 31, 10, 0)))
}

func TestCUJR2SpecificThenMixedHistory(t *testing.T) {
	e := newDueEnv(t)
	acrilico := e.service(t, "Acrílico", 60, nil)
	cejas := e.service(t, "Cejas", 30, nil)
	specific := e.rule(t, RuleInput{
		Kind: KindComeBack, ServiceID: nu(acrilico), OffsetDays: 21, IncludeOverdue: true,
		TemplateID: e.template(t, "Uñas", "Hola {{client.first_name}}, ¿tus uñas?"),
	})
	g := e.general(t, 30)
	ana := e.client(t, "Ana Rojas", true)
	nails := e.book(t, ana.ID, e.at(2026, 10, 1, 10, 0), acrilico)
	brows := e.book(t, ana.ID, e.at(2026, 10, 5, 10, 0), cejas)

	items := e.dueAt(t, e.at(2026, 11, 4, 9, 0))
	if len(items) != 2 {
		t.Fatalf("got %d items %+v, want the nails item and the brows item", len(items), items)
	}
	nailsItem, browsItem := items[0], items[1]
	if nailsItem.RuleID != specific.ID || nailsItem.AppointmentID != nails.ID || nailsItem.General || !nailsItem.Late {
		t.Fatalf("first = %+v, want the late specific item of the nails visit", nailsItem)
	}
	if browsItem.RuleID != g.ID || browsItem.AppointmentID != brows.ID || !browsItem.General {
		t.Fatalf("second = %+v, want the general item of the brows visit", browsItem)
	}
}

func TestCUJR3SecondGeneralMessageIsNumbered(t *testing.T) {
	e := newDueEnv(t)
	acrilico := e.service(t, "Acrílico", 60, nil)
	g30, g45 := e.general(t, 30), e.general(t, 45)
	ana := e.client(t, "Ana Rojas", true)
	visit := e.book(t, ana.ID, e.at(2026, 10, 1, 10, 0), acrilico)

	first := one(t, e.dueAt(t, e.at(2026, 10, 31, 9, 0)))
	if first.RuleID != g30.ID || first.MessageNumber != 1 || first.NextDueDate == nil || !first.NextDueDate.Equal(e.at(2026, 11, 15, 0, 0)) {
		t.Fatalf("31 Oct = %+v, want the 30-day item, message 1, next 15 Nov", first)
	}
	e.record(t, LogEntry{RuleID: g30.ID, AppointmentID: visit.ID, Action: ActionSent, ActedAt: e.at(2026, 10, 31, 9, 5)})
	none(t, e.dueAt(t, e.at(2026, 11, 1, 9, 0)))

	second := one(t, e.dueAt(t, e.at(2026, 11, 15, 9, 0)))
	if second.RuleID != g45.ID || second.MessageNumber != 2 {
		t.Fatalf("15 Nov = %+v, want the 45-day item as message 2", second)
	}
}

func TestCUJR4SpecificRuleStopsThenComesBack(t *testing.T) {
	for _, tc := range []struct {
		name    string
		include bool
		general bool
	}{
		{"from today: the late general item stays hidden", false, false},
		{"include late: the general item shows late", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newDueEnv(t)
			acrilico := e.service(t, "Acrílico", 60, nil)
			specific := e.rule(t, RuleInput{
				Kind: KindComeBack, ServiceID: nu(acrilico), OffsetDays: 21, IncludeOverdue: true,
				TemplateID: e.template(t, "Uñas", "Hola {{client.first_name}}, ¿tus uñas?"),
			})
			e.general(t, 30)
			ana := e.client(t, "Ana Rojas", true)
			e.book(t, ana.ID, e.at(2026, 10, 1, 10, 0), acrilico)

			if it := one(t, e.dueAt(t, e.at(2026, 11, 2, 9, 0))); it.RuleID != specific.ID {
				t.Fatalf("before the archive: %+v, want the specific item (the general one is held back)", it)
			}
			e.clock.Set(e.at(2026, 11, 2, 9, 0))
			if _, err := e.m.UpdateRule(t.Context(), e.pool, specific.ID, RulePatch{Archived: ptr(true), IncludeOverdue: tc.include}); err != nil {
				t.Fatalf("archive: %v", err)
			}
			items := e.dueAt(t, e.at(2026, 11, 2, 9, 30))
			if got := len(items) == 1 && items[0].General && items[0].Late; got != tc.general {
				t.Fatalf("after the archive: %+v, want a late general item = %v", items, tc.general)
			}

			// Restoring the rule hands the visit back to it.
			if _, err := e.m.UpdateRule(t.Context(), e.pool, specific.ID, RulePatch{Archived: ptr(false), IncludeOverdue: true}); err != nil {
				t.Fatalf("restore: %v", err)
			}
			if it := one(t, e.dueAt(t, e.at(2026, 11, 2, 10, 0))); it.RuleID != specific.ID {
				t.Fatalf("after the restore: %+v, want the specific item", it)
			}
		})
	}
}
