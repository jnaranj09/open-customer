package reminders

import (
	"cmp"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/locale"
)

// The look-ahead pass (reminders-and-replies design, "Look-ahead pass"; item 42).

// client makes a the visit of another made-up client.
func client(a apptRow, name string) apptRow {
	a.ClientID, a.ClientName = uuid.New(), name
	return a
}

// itemKey is what makes two items the same reminder.
type itemKey struct {
	Rule, Appointment, Client uuid.UUID
	Anchor                    int64
}

func keysDueOn(items []DueItem, due time.Time) []itemKey {
	var out []itemKey
	for _, it := range items {
		if !it.DueDate.Equal(due) {
			continue
		}
		k := itemKey{Rule: it.RuleID, Appointment: it.AppointmentID, Client: it.ClientID}
		if it.AnchorAt != nil {
			k.Anchor = it.AnchorAt.Unix()
		}
		out = append(out, k)
	}
	slices.SortFunc(out, func(x, y itemKey) int {
		return cmp.Or(slices.Compare(x.Rule[:], y.Rule[:]), slices.Compare(x.Appointment[:], y.Appointment[:]), cmp.Compare(x.Anchor, y.Anchor))
	})
	return out
}

func TestUpcomingAgreesWithTheDueListOnEachDay(t *testing.T) {
	b := newDue(t)
	now := b.at(2026, 11, 2, 9, 0)
	visit := func(m time.Month, d int, svcs ...service) apptRow {
		return appt(b.at(2026, m, d, 10, 0), b.at(2026, m, d, 11, 0), b.at(2026, m, d, 9, 0), svcs...)
	}
	specific := b.comeBackRule(acrilico, acrilico, 21)
	g30 := b.generalRule(30)
	g45 := b.generalRule(45)
	b.generalRule(45) // a tie on the same day: one item, the higher id
	stopped := b.comeBackRule(pedicure, pedicure, 10)
	stopped.Archived, stopped.HandoverFrom = true, new(b.at(2026, 11, 5, 12, 0))
	b.d.Rules[len(b.d.Rules)-1] = stopped
	before := b.beforeRule(1)
	b.beforeRule(0)

	b.comeBack(client(visit(10, 20, acrilico), "Ana")) // the specific rule has her visit (G2)
	bea := b.comeBack(client(visit(10, 15, relleno), "Bea"))
	b.logComeBack(g45, bea) // logged ahead (an offset changed after a send): never listed
	carla := client(visit(9, 10, relleno), "Carla")
	b.comeBack(carla)
	latest := visit(10, 1, pedicure) // the cutoff of the stopped rule hides due days before 5 Nov
	latest.ClientID, latest.ClientName = carla.ClientID, carla.ClientName
	b.comeBack(latest)
	dora := b.comeBack(client(visit(10, 25, relleno), "Dora"))
	b.later(dora, b.at(2027, 3, 1, 10, 0), relleno) // a later booking (G1)
	eva := b.comeBack(client(visit(10, 28, acrilico), "Eva"))
	b.later(eva, b.at(2026, 12, 20, 10, 0), relleno) // a booking without the return service
	fina := client(visit(11, 10, relleno), "Fina")
	finaLater := visit(12, 2, relleno)
	finaLater.ClientID, finaLater.ClientName = fina.ClientID, fina.ClientName
	fina.CreatedAt, finaLater.CreatedAt = now, now // booked today, before both due days
	b.before(fina)
	b.before(finaLater)
	b.logBefore(before, finaLater)

	today := locale.LocalDay(now, b.d.TZ)
	ahead := upcoming(b.d, now, locale.AddDays(today, maxOffsetDays, b.d.TZ))
	rules := map[uuid.UUID]bool{}
	for _, it := range ahead {
		rules[it.RuleID] = true
	}
	if !rules[specific.ID] || !rules[g30.ID] || !rules[before.ID] {
		t.Fatalf("upcoming rules = %v, want the specific, the general and the before rule", rules)
	}
	for n := 1; n <= 90; n++ {
		due := locale.AddDays(today, n, b.d.TZ)
		want := keysDueOn(compute(b.d, due), due)
		if got := keysDueOn(ahead, due); !slices.Equal(got, want) {
			t.Fatalf("%s: upcoming %+v, due list %+v", due.Format(time.DateOnly), got, want)
		}
	}
}

func TestUpcomingListsEachLaterGeneralRuleOnItsDay(t *testing.T) {
	b := newDue(t)
	g30, g45 := b.generalRule(30), b.generalRule(45)
	b.visit(acrilico)
	now := b.at(2026, 10, 20, 9, 0)
	items := upcoming(b.d, now, b.dayOf(12, 31))
	if len(items) != 2 || items[0].RuleID != g30.ID || items[1].RuleID != g45.ID {
		t.Fatalf("items = %+v, want the 30-day rule on 31 Oct, then the 45-day rule", items)
	}
	day(t, items[0], b.dayOf(10, 31))
	if items[0].NextDueDate == nil || !items[0].NextDueDate.Equal(b.dayOf(11, 15)) {
		t.Fatalf("next = %v, want 15 Nov", items[0].NextDueDate)
	}
}

func TestUpcomingStopsAtTheHorizonAndSkipsToday(t *testing.T) {
	b := newDue(t)
	b.generalRule(30)
	b.visit(acrilico)
	none(t, upcoming(b.d, b.at(2026, 10, 20, 9, 0), b.dayOf(10, 30)))
	none(t, upcoming(b.d, b.at(2026, 10, 31, 9, 0), b.dayOf(12, 31)))
}
