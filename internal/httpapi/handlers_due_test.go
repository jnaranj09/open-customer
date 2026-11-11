//go:build db

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

const duePath = "/api/v1/reminders/due"

// dueItemKeys are the keys of a DueItem.
var dueItemKeys = []string{
	"anchor_at", "appointment_id", "client_id", "client_name", "due_date", "general", "id", "kind",
	"late", "message_number", "mode", "next_due_date", "offset_days", "phone_digits", "prompt",
	"rule_id", "shown_until", "source_services", "source_starts_at", "text", "wa_link",
}

// bookAt books an appointment of client for the service at start (an RFC 3339 time), allowing an
// overlap, and returns it.
func bookAt(t *testing.T, app testApp, c *http.Cookie, client gen.Client, service gen.Service, start string) gen.Appointment {
	t.Helper()
	body := fmt.Sprintf(`{"client_id":%q,"starts_at":%q,"service_ids":[%q],"allow_overlap":true}`, client.Id, start, service.Id)
	return decodeInto[gen.Appointment](t, sendAs(app, c, http.MethodPost, appointmentsPath, body), http.StatusCreated)
}

// TestGetRemindersDueReturnsCUJ4And5Items is the HTTP half of the P4 exit check: the rows of
// CUJ 4 (the "before" reminder) and CUJ 5 (the come-back message, and the booking prompt for a
// client who already has a booking without the return service), made through the API, give
// exactly those three items. internal/reminders/cuj_db_test.go is the module half.
func TestGetRemindersDueReturnsCUJ4And5Items(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	// The session must be created at the first business day below, or it would expire on the way.
	app.clock.Set(time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC))
	c := sessionCookie(completeSetup(t, app, pool, testOrigin))
	if c == nil {
		t.Fatal("setup set no session cookie")
	}
	acrilico := createService(t, app, c, `{"name":"Acrílico","duration_min":60,"color":"#AA3366"}`)
	relleno := createService(t, app, c, `{"name":"Relleno","duration_min":45,"color":"#336699"}`)
	pedicure := createService(t, app, c, `{"name":"Pedicure","duration_min":30,"color":"#339966"}`)
	before := createTemplate(t, app, c, `{"name":"Recordatorio de cita","body":"Hola {{client.first_name}}, te esperamos {{appointment.date}} a las {{appointment.time}}"}`)
	back := createTemplate(t, app, c, `{"name":"Relleno","body":"Hola {{client.first_name}}, ¿agendamos tu relleno?"}`)
	createRule(t, app, c, `{"kind":"before_appointment","offset_days":1,"include_overdue":true,"template_id":"`+before.Id.String()+`"}`)
	createRule(t, app, c, fmt.Sprintf(`{"kind":"come_back","offset_days":21,"include_overdue":true,"template_id":%q,"service_id":%q,"return_service_id":%q}`,
		back.Id, acrilico.Id, relleno.Id))
	ana := createClient(t, app, c, `{"name":"Ana Rojas","phone":"612 345 678"}`)
	marta := createClient(t, app, c, `{"name":"Marta Solano"}`)
	lucia := createClient(t, app, c, `{"name":"Lucía Rojas"}`)

	martaVisit := bookAt(t, app, c, marta, acrilico, "2026-10-01T15:00:00+02:00")
	luciaVisit := bookAt(t, app, c, lucia, acrilico, "2026-10-01T15:00:00+02:00")
	luciaNext := bookAt(t, app, c, lucia, pedicure, "2026-11-12T10:00:00+01:00")
	app.clock.Set(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
	anaTomorrow := bookAt(t, app, c, ana, pedicure, "2026-10-23T15:00:00+02:00")
	app.clock.Set(time.Date(2026, 10, 22, 8, 0, 0, 0, time.FixedZone("CEST", 2*3600)))

	rec := sendAs(app, c, http.MethodGet, duePath, "")
	var raw struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil || len(raw.Items) != 3 {
		t.Fatalf("status %d, body %q; want 3 items (%v)", rec.Code, rec.Body.String(), err)
	}
	for _, item := range raw.Items {
		assertKeys(t, item, dueItemKeys...)
		if !strings.Contains(string(item), `"due_date":"2026-10-22"`) {
			t.Fatalf("item %s; want due_date 2026-10-22", item)
		}
	}
	got := decodeInto[gen.DueList](t, rec, http.StatusOK).Items
	anaItem, luciaItem, martaItem := got[0], got[1], got[2]

	wantText := "Hola Ana, te esperamos viernes 23 de octubre a las 3:00 p. m."
	if anaItem.Kind != "before_appointment" || anaItem.AppointmentId != anaTomorrow.Id || anaItem.Text != wantText || anaItem.Mode != "message" {
		t.Fatalf("first item = %+v; want Ana's before message %q", anaItem, wantText)
	}
	if link, err := anaItem.WaLink.Get(); err != nil || !strings.HasPrefix(link, "https://wa.me/34612345678?text=Hola%20Ana") {
		t.Fatalf("Ana's wa_link = %q, %v; want a wa.me link to her number", link, err)
	}
	if !strings.Contains(string(raw.Items[0]), `"prompt":null`) {
		t.Fatalf("first item %s; want prompt null", raw.Items[0])
	}

	prompt, err := luciaItem.Prompt.Get()
	if err != nil || luciaItem.Mode != "booking_prompt" || luciaItem.Kind != "come_back" || luciaItem.AppointmentId != luciaVisit.Id {
		t.Fatalf("second item = %+v (%v); want Lucía's booking prompt", luciaItem, err)
	}
	if prompt.AppointmentId != luciaNext.Id || prompt.ReturnServiceName != "Relleno" || prompt.ReturnServiceId != relleno.Id ||
		len(prompt.Services) != 1 || prompt.Services[0] != "Pedicure" {
		t.Fatalf("Lucía's prompt = %+v; want booking %s (Pedicure), return service Relleno", prompt, luciaNext.Id)
	}

	if martaItem.Kind != "come_back" || martaItem.AppointmentId != martaVisit.Id || martaItem.Mode != "message" ||
		martaItem.Text != "Hola Marta, ¿agendamos tu relleno?" || martaItem.Late {
		t.Fatalf("third item = %+v; want Marta's come-back message, due today", martaItem)
	}
	if !strings.Contains(string(raw.Items[2]), `"prompt":null`) {
		t.Fatalf("third item %s; want prompt null", raw.Items[2])
	}
}

func TestGetRemindersDueEmptyIsEmptyList(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodGet, duePath, "")
	if body := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || body != `{"items":[]}` {
		t.Fatalf("status %d body %q; want 200 {\"items\":[]}", rec.Code, body)
	}
}
