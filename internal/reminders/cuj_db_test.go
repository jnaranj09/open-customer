//go:build db

package reminders

import "testing"

// TestDueListCUJ4AndCUJ5 is the P4 exit test (design doc "Build order"): the rows of CUJ 4 (the
// "before" reminder) and CUJ 5 (the come-back message, and the booking prompt for a client who
// already has a booking without the return service), made with the real modules, give exactly
// those three items in the due-list order.
func TestDueListCUJ4AndCUJ5(t *testing.T) {
	e := newDueEnv(t)
	acrilico := e.service(t, "Acrílico", 60, nil)
	relleno := e.service(t, "Relleno", 45, nil)
	pedicure := e.service(t, "Pedicure", 30, nil)
	e.rule(t, RuleInput{
		Kind: KindBefore, OffsetDays: 1, IncludeOverdue: true,
		TemplateID: e.template(t, "Recordatorio de cita", "Hola {{client.first_name}}, te esperamos {{appointment.date}} a las {{appointment.time}}"),
	})
	e.rule(t, RuleInput{
		Kind: KindComeBack, ServiceID: nu(acrilico), ReturnServiceID: nu(relleno), OffsetDays: 21, IncludeOverdue: true,
		TemplateID: e.template(t, "Relleno", "Hola {{client.first_name}}, ¿agendamos tu relleno?"),
	})
	ana := e.client(t, "Ana Rojas", true)
	marta := e.client(t, "Marta Solano", false)
	lucia := e.client(t, "Lucía Rojas", false)
	martaVisit := e.book(t, marta.ID, e.at(2026, 10, 1, 15, 0), acrilico) // created 2026-09-30
	luciaVisit := e.book(t, lucia.ID, e.at(2026, 10, 1, 15, 0), acrilico)
	luciaNext := e.book(t, lucia.ID, e.at(2026, 11, 12, 10, 0), pedicure)
	e.clock.Set(e.at(2026, 10, 20, 12, 0))
	anaTomorrow := e.book(t, ana.ID, e.at(2026, 10, 23, 15, 0), pedicure)
	e.clock.Set(e.now())

	items := e.due(t, e.pool)
	if len(items) != 3 {
		t.Fatalf("got %d items %+v; want 3", len(items), items)
	}
	anaItem, luciaItem, martaItem := items[0], items[1], items[2]

	wantText := "Hola Ana, te esperamos viernes 23 de octubre a las 3:00 p. m."
	if anaItem.Kind != KindBefore || anaItem.AppointmentID != anaTomorrow.ID || anaItem.Text != wantText {
		t.Fatalf("first item = %+v; want Ana's before item %q", anaItem, wantText)
	}
	if anaItem.WaLink == nil || *anaItem.WaLink != WhatsAppLink(*ana.PhoneDigits, wantText) {
		t.Fatalf("Ana's link = %v; want the wa.me link to %s", anaItem.WaLink, *ana.PhoneDigits)
	}

	if luciaItem.Kind != KindComeBack || luciaItem.AppointmentID != luciaVisit.ID || luciaItem.Mode != ModeBookingPrompt {
		t.Fatalf("second item = %+v; want Lucía's booking prompt", luciaItem)
	}
	p := luciaItem.Prompt
	if p == nil || p.AppointmentID != luciaNext.ID || p.ReturnServiceName != "Relleno" || p.ReturnServiceID != relleno ||
		len(p.Services) != 1 || p.Services[0] != "Pedicure" {
		t.Fatalf("Lucía's prompt = %+v; want booking %s (Pedicure), return service Relleno", p, luciaNext.ID)
	}

	if martaItem.Kind != KindComeBack || martaItem.AppointmentID != martaVisit.ID || martaItem.Mode != ModeMessage ||
		martaItem.Text != "Hola Marta, ¿agendamos tu relleno?" || martaItem.Late {
		t.Fatalf("third item = %+v; want Marta's come-back message, due today", martaItem)
	}
	for _, it := range items {
		if !it.DueDate.Equal(e.at(2026, 10, 22, 0, 0)) {
			t.Fatalf("item %s due %v; want 2026-10-22", it.ID, it.DueDate)
		}
	}
}
