package reminders

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/locale"
)

// General rules in compute (reminders-and-replies design, "Due-list rules" and "New fields on each
// due item"; item 27). Ana's visit is 1 Oct, 10:00-11:00 Madrid, booked 20 Sep.

// generalRule adds a general rule (a come_back rule with no service) with offset days.
func (b *dueBuilder) generalRule(offset int) ruleRow {
	r := ruleRow{
		ID: uuid.New(), Kind: KindComeBack, OffsetDays: offset,
		TemplateBody: "Hola {{client.first_name}}, ¿te agendo?",
	}
	b.d.Rules = append(b.d.Rules, r)
	return r
}

// sent records sent come-back messages for visit a; general marks one of them as a general one.
func (b *dueBuilder) sent(a apptRow, n int, general bool) {
	if b.d.SentByVisit == nil {
		b.d.SentByVisit, b.d.GeneralSent = map[uuid.UUID]int{}, map[uuid.UUID]bool{}
	}
	b.d.SentByVisit[a.ID] += n
	b.d.GeneralSent[a.ID] = b.d.GeneralSent[a.ID] || general
}

func (b *dueBuilder) visit(svcs ...service) apptRow {
	return b.comeBack(appt(b.at(2026, 10, 1, 10, 0), b.at(2026, 10, 1, 11, 0), b.at(2026, 9, 20, 9, 0), svcs...))
}

func (b *dueBuilder) dayOf(m time.Month, d int) time.Time { return b.at(2026, m, d, 0, 0) }

func TestGeneralRuleCoversEveryService(t *testing.T) {
	b := newDue(t)
	g := b.generalRule(30)
	v := b.visit(acrilico, pedicure)
	it := one(t, b.run(b.at(2026, 10, 31, 9, 0)))
	day(t, it, b.dayOf(10, 31))
	if it.RuleID != g.ID || it.AppointmentID != v.ID || !it.General || it.OffsetDays != 30 || it.MessageNumber != 1 {
		t.Fatalf("item = %+v, want general rule 30 for the visit, message 1", it)
	}
	if !it.SourceStartsAt.Equal(v.StartsAt) || !slices.Equal(it.SourceServices, []string{"Acrílico", "Pedicure"}) {
		t.Fatalf("source = %v %v, want the visit", it.SourceStartsAt, it.SourceServices)
	}
	if !it.ShownUntil.Equal(b.dayOf(12, 30)) || it.NextDueDate != nil {
		t.Fatalf("shown_until %v, next %v; want 30 Dec and none", it.ShownUntil, it.NextDueDate)
	}
}

func TestGeneralRuleNotYetDue(t *testing.T) {
	b := newDue(t)
	b.generalRule(30)
	b.visit(acrilico)
	none(t, b.run(b.at(2026, 10, 30, 9, 0)))
}

func TestTwoGeneralRulesShowTheLatestDueOnce(t *testing.T) {
	b := newDue(t)
	g30, g45 := b.generalRule(30), b.generalRule(45)
	b.visit(acrilico)

	it := one(t, b.run(b.at(2026, 10, 31, 9, 0)))
	if it.RuleID != g30.ID || it.NextDueDate == nil || !it.NextDueDate.Equal(b.dayOf(11, 15)) {
		t.Fatalf("31 Oct: rule %v, next %v; want the 30-day rule and 15 Nov", it.RuleID, it.NextDueDate)
	}
	// quedan 14: the day before the 45-day rule is due.
	if !it.ShownUntil.Equal(b.dayOf(11, 14)) {
		t.Fatalf("shown_until = %v, want 14 Nov", it.ShownUntil)
	}

	it = one(t, b.run(b.at(2026, 11, 15, 9, 0)))
	if it.RuleID != g45.ID || it.NextDueDate != nil || !it.ShownUntil.Equal(time.Date(2027, 1, 14, 0, 0, 0, 0, b.d.TZ)) {
		t.Fatalf("15 Nov: %+v, want only the 45-day rule, no next, until 14 Jan", it)
	}
}

func TestGeneralLoggedLatestHidesTheEarlierOnes(t *testing.T) {
	b := newDue(t)
	b.generalRule(30)
	g45 := b.generalRule(45)
	v := b.visit(acrilico)
	b.logComeBack(g45, v)
	none(t, b.run(b.at(2026, 11, 15, 9, 0)))
}

func TestGeneralTieTakesTheLargerOffset(t *testing.T) {
	b := newDue(t)
	b.generalRule(30)
	g31 := b.generalRule(31)
	b.visit(acrilico)
	// 30 days after 1 Oct is 31 Oct; 31 days after is 1 Nov. Both due by 2 Nov: the later due wins.
	if it := one(t, b.run(b.at(2026, 11, 2, 9, 0))); it.RuleID != g31.ID {
		t.Fatalf("rule = %v, want the 31-day rule", it.RuleID)
	}
}

func TestGeneralUsesTheLatestVisitAndASameStartHigherIDIsLater(t *testing.T) {
	b := newDue(t)
	b.generalRule(30)
	low := appt(b.at(2026, 10, 1, 10, 0), b.at(2026, 10, 1, 11, 0), b.at(2026, 9, 20, 9, 0), acrilico)
	high := low
	low.ID = uuid.MustParse("019a0000-0000-7000-8000-000000000100")
	high.ID = uuid.MustParse("019a0000-0000-7000-8000-000000000200")
	high.ServiceIDs, high.ServiceNames = []uuid.UUID{pedicure.ID}, []string{"Pedicure"}
	b.comeBack(high)
	b.comeBack(low)
	it := one(t, b.run(b.at(2026, 10, 31, 9, 0)))
	if it.AppointmentID != high.ID {
		t.Fatalf("visit = %v, want the higher id", it.AppointmentID)
	}

	b = newDue(t)
	b.generalRule(30)
	b.comeBack(low)
	l := b.later(low, low.StartsAt)
	l.ID = high.ID
	b.d.Later[0] = l
	none(t, b.run(b.at(2026, 10, 31, 9, 0)))
}

func TestGeneralHiddenByAnyLaterAppointment(t *testing.T) {
	b := newDue(t)
	b.generalRule(30)
	v := b.visit(acrilico)
	b.later(v, b.at(2026, 11, 20, 10, 0), pedicure)
	none(t, b.run(b.at(2026, 10, 31, 9, 0)))
}

func TestGeneralHiddenWhileASpecificRuleCanSend(t *testing.T) {
	b := newDue(t)
	b.generalRule(30)
	b.comeBackRule(pedicure, pedicure, 50)
	b.visit(acrilico, pedicure)
	// The specific rule is not due yet, and still it holds the visit (G2).
	none(t, b.run(b.at(2026, 10, 31, 9, 0)))
}

func TestGeneralShowsWhenTheSpecificRuleCannotSend(t *testing.T) {
	for _, tc := range []struct {
		name string
		stop func(*ruleRow)
	}{
		{"rule archived", func(r *ruleRow) { r.Archived = true }},
		{"template archived", func(r *ruleRow) { r.TemplateArchived = true }},
		{"service archived", func(r *ruleRow) { r.ServiceArchived = true }},
		{"return service archived", func(r *ruleRow) { r.ReturnArchived = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newDue(t)
			g := b.generalRule(30)
			b.comeBackRule(acrilico, acrilico, 21)
			tc.stop(&b.d.Rules[1])
			b.visit(acrilico)
			if it := one(t, b.run(b.at(2026, 10, 31, 9, 0))); it.RuleID != g.ID {
				t.Fatalf("rule = %v, want the general rule", it.RuleID)
			}
		})
	}
}

func TestGeneralCutoff(t *testing.T) {
	for _, tc := range []struct {
		name     string
		handover []*time.Time
		show     bool
	}{
		{"no answer: late visits come", []*time.Time{nil}, true},
		{"from a day after the due day: hidden", []*time.Time{ptrTime(time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC))}, false},
		{"from the due day: shown", []*time.Time{ptrTime(time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC))}, true},
		{"the latest of two wins", []*time.Time{ptrTime(time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)), ptrTime(time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC))}, false},
		{"an answer wins over no answer", []*time.Time{nil, ptrTime(time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC))}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newDue(t)
			b.generalRule(30)
			for _, h := range tc.handover {
				r := b.comeBackRule(acrilico, acrilico, 21)
				r.Archived, r.HandoverFrom = true, h
				b.d.Rules[len(b.d.Rules)-1] = r
			}
			b.visit(acrilico)
			items := b.run(b.at(2026, 11, 2, 9, 0))
			if got := len(items) == 1; got != tc.show {
				t.Fatalf("shown = %v, want %v (%+v)", got, tc.show, items)
			}
		})
	}
}

func TestGeneralNextDueDateIsTheEarliestAhead(t *testing.T) {
	b := newDue(t)
	b.generalRule(30)
	b.generalRule(60)
	b.generalRule(45)
	b.visit(acrilico)
	it := one(t, b.run(b.at(2026, 10, 31, 9, 0)))
	if it.NextDueDate == nil || !it.NextDueDate.Equal(b.dayOf(11, 15)) {
		t.Fatalf("next = %v, want 15 Nov", it.NextDueDate)
	}
}

func TestSpecificHiddenOnceAGeneralWasSent(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, acrilico, 21)
	v := b.visit(acrilico)
	b.sent(v, 1, true)
	none(t, b.run(b.at(2026, 10, 25, 9, 0)))
}

func TestSpecificShownUntilAndMessageNumber(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, acrilico, 21)
	b.comeBackRule(acrilico, acrilico, 10)
	v := b.visit(acrilico)
	// The 10-day message was sent (it is logged); a skipped one would not count.
	b.logComeBack(b.d.Rules[1], v)
	b.sent(v, 1, false)
	it := one(t, b.run(b.at(2026, 10, 25, 9, 0)))
	if it.MessageNumber != 2 || it.General || !it.ShownUntil.Equal(locale.AddDays(b.dayOf(10, 22), lookBackDays, b.d.TZ)) {
		t.Fatalf("item = %+v, want message 2, specific, shown until due + 60", it)
	}
}

func TestBeforeShownUntilTheAppointmentDay(t *testing.T) {
	b := newDue(t)
	b.beforeRule(2)
	b.before(appt(b.at(2026, 11, 3, 10, 0), b.at(2026, 11, 3, 11, 0), b.at(2026, 10, 1, 9, 0), acrilico))
	it := one(t, b.run(b.at(2026, 11, 1, 9, 0)))
	if it.MessageNumber != 1 || !it.ShownUntil.Equal(b.dayOf(11, 3)) || it.General {
		t.Fatalf("item = %+v, want message 1 shown until 3 Nov", it)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
