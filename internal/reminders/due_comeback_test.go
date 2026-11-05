package reminders

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/appointments"
)

// acrylicVisit adds CUJ 5's visit: Acrílico on 2026-10-01, ending at 16:00, booked days before.
func acrylicVisit(b *dueBuilder) apptRow {
	return b.comeBack(appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 16, 0), b.at(2026, 9, 28, 10, 0), acrilico))
}

func TestComeBackDueOnDayNWithoutTap(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	acrylicVisit(b)

	t.Run("not on the day before", func(t *testing.T) {
		none(t, b.run(b.at(2026, 10, 21, 23, 59)))
	})
	t.Run("on day N with the visit still scheduled", func(t *testing.T) {
		it := one(t, b.run(b.at(2026, 10, 22, 12, 0)))
		if it.Mode != ModeMessage || it.Late {
			t.Fatalf("mode %q late %v, want a message on time", it.Mode, it.Late)
		}
		day(t, it, b.at(2026, 10, 22, 0, 0))
	})
}

func TestComeBackCountsAtOneMinutePastMidnight(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	acrylicVisit(b)

	one(t, b.run(b.at(2026, 10, 22, 0, 1)))
}

func TestComeBackNoShowRemoves(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	a := appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 16, 0), b.at(2026, 9, 28, 10, 0), acrilico)
	a.Status = appointments.NoShow
	b.comeBack(a)

	none(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestComeBackSkippedByLaterBookingWithReturnService(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	a := acrylicVisit(b)
	b.later(a, b.at(2026, 10, 30, 10, 0), relleno)

	none(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestComeBackNotSkippedByReturnServiceStartingBeforeEnd(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	a := acrylicVisit(b)
	b.later(a, b.at(2026, 10, 1, 15, 30), relleno)

	one(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestComeBackNotSkippedByOtherClientsBooking(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	a := acrylicVisit(b)
	other := a
	other.ClientID = uuid.MustParse("019a0000-0000-7000-8000-0000000000b2")
	b.later(other, b.at(2026, 10, 30, 10, 0), relleno)

	one(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestComeBackNotSkippedByLaterVisitWithoutReturnService(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	a := acrylicVisit(b)
	b.later(a, b.at(2026, 10, 10, 10, 0), pedicure)

	if it := one(t, b.run(b.at(2026, 10, 22, 8, 0))); it.Mode != ModeMessage {
		t.Fatalf("mode %q, want message", it.Mode)
	}
}

func TestEmptyReturnServiceMeansOwnService(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, acrilico, 21)
	a := acrylicVisit(b)
	b.later(a, b.at(2026, 10, 30, 10, 0), acrilico)

	none(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestBookingPromptForFutureBookingWithoutReturnService(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	a := acrylicVisit(b)
	l := b.later(a, b.at(2026, 11, 12, 10, 0), pedicure)

	it := one(t, b.run(b.at(2026, 10, 22, 8, 0)))
	if it.Mode != ModeBookingPrompt || it.Prompt == nil {
		t.Fatalf("mode %q prompt %v, want a booking prompt", it.Mode, it.Prompt)
	}
	p := *it.Prompt
	if p.AppointmentID != l.ID || !p.StartsAt.Equal(l.StartsAt) || !slices.Equal(p.Services, []string{"Pedicure"}) {
		t.Fatalf("prompt booking %+v, want %s at %v (Pedicure)", p, l.ID, l.StartsAt)
	}
	if p.ReturnServiceID != relleno.ID || p.ReturnServiceName != "Relleno" {
		t.Fatalf("prompt return service %s %q, want Relleno", p.ReturnServiceID, p.ReturnServiceName)
	}
	if want := "Hola Ana, ¿reservamos tu Acrílico?"; it.Text != want || it.WaLink == nil || *it.WaLink != WhatsAppLink(anaPhone, want) {
		t.Fatalf("text %q link %v, want %q and its link", it.Text, it.WaLink, want)
	}
}

func TestBookingPromptUsesEarliestFutureBooking(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	a := acrylicVisit(b)
	b.later(a, b.at(2026, 11, 20, 10, 0), pedicure)
	first := b.later(a, b.at(2026, 11, 12, 10, 0), pedicure)

	it := one(t, b.run(b.at(2026, 10, 22, 8, 0)))
	if it.Prompt == nil || it.Prompt.AppointmentID != first.ID {
		t.Fatalf("prompt %+v, want the booking %s", it.Prompt, first.ID)
	}
}

func TestLaterBookingBeforeDueDayGivesMessage(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	a := acrylicVisit(b)
	b.later(a, b.at(2026, 10, 15, 10, 0), pedicure)

	it := one(t, b.run(b.at(2026, 10, 22, 8, 0)))
	if it.Mode != ModeMessage || it.Prompt != nil {
		t.Fatalf("mode %q prompt %v, want a message", it.Mode, it.Prompt)
	}
}

func TestHandledPromptStaysHidden(t *testing.T) {
	b := newDue(t)
	r := b.comeBackRule(acrilico, relleno, 21)
	a := acrylicVisit(b)
	b.later(a, b.at(2026, 11, 12, 10, 0), pedicure)
	b.logComeBack(r, a)

	none(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestTwoComeBackRulesEachCheckOwnReturnService(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	pedi := b.comeBackRule(pedicure, pedicure, 21)
	a := b.comeBack(appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 17, 0), b.at(2026, 9, 28, 10, 0), acrilico, pedicure))
	b.later(a, b.at(2026, 10, 10, 10, 0), relleno)

	if it := one(t, b.run(b.at(2026, 10, 22, 8, 0))); it.RuleID != pedi.ID {
		t.Fatalf("item of rule %s, want the Pedicure rule %s", it.RuleID, pedi.ID)
	}
}

func TestComeBackRuleServiceMustBeOnAppointment(t *testing.T) {
	b := newDue(t)
	b.comeBackRule(acrilico, relleno, 21)
	b.comeBack(appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 16, 0), b.at(2026, 9, 28, 10, 0), pedicure))

	none(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestComeBackHandledStaysHiddenAfterEndsAtChanges(t *testing.T) {
	b := newDue(t)
	r := b.comeBackRule(acrilico, relleno, 21)
	a := appt(b.at(2026, 10, 1, 15, 0), b.at(2026, 10, 1, 16, 0), b.at(2026, 9, 28, 10, 0), acrilico)
	b.logComeBack(r, a)
	a.EndsAt = b.at(2026, 10, 1, 17, 0)
	b.comeBack(a)

	none(t, b.run(b.at(2026, 10, 22, 8, 0)))
}

func TestPastVisitTypedToday(t *testing.T) {
	now := time.Date(2026, 10, 22, 8, 0, 0, 0, testZone(t))
	visit := func(b *dueBuilder, daysAgo int) {
		end := now.AddDate(0, 0, -daysAgo)
		b.comeBack(appt(end.Add(-time.Hour), end, now.Add(-time.Hour), acrilico))
	}

	t.Run("ended 40 days ago gives no item", func(t *testing.T) {
		b := newDue(t)
		b.comeBackRule(acrilico, relleno, 21)
		visit(b, 40)
		none(t, b.run(now))
	})
	t.Run("ended 10 days ago gives none today", func(t *testing.T) {
		b := newDue(t)
		b.comeBackRule(acrilico, relleno, 21)
		visit(b, 10)
		none(t, b.run(now))
	})
	t.Run("ended 10 days ago gives one in 11 days", func(t *testing.T) {
		b := newDue(t)
		b.comeBackRule(acrilico, relleno, 21)
		visit(b, 10)
		day(t, one(t, b.run(now.AddDate(0, 0, 11))), b.at(2026, 11, 2, 0, 0))
	})
}
