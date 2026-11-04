//go:build db

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

const appointmentsPath = "/api/v1/appointments"

// appointmentKeys are the keys of an Appointment.
var appointmentKeys = []string{
	"attrs", "client_id", "client_name", "created_at", "done", "ends_at", "id", "notes", "paid_cents",
	"services", "starts_at", "status", "total_cents", "updated_at",
}

// bookingFixture is a signed-in app with one client ("Lucía Rojas") and one 90-minute service
// priced 2500.
type bookingFixture struct {
	app     testApp
	cookie  *http.Cookie
	client  gen.Client
	service gen.Service
}

func newBookingFixture(t *testing.T) bookingFixture {
	t.Helper()
	app, _, c := signedInOwner(t)
	return bookingFixture{
		app: app, cookie: c,
		client:  createClient(t, app, c, clientBody),
		service: createService(t, app, c, `{"name":"Acrílico","duration_min":90,"price_cents":2500,"color":"#AA3366"}`),
	}
}

func (f bookingFixture) send(method, target, body string) *httptest.ResponseRecorder {
	return sendAs(f.app, f.cookie, method, target, body)
}

// bookBody is a POST /appointments body for the fixture's client and service at start.
func (f bookingFixture) bookBody(start string, extra string) string {
	return fmt.Sprintf(`{"client_id":%q,"starts_at":%q,"service_ids":[%q]%s}`, f.client.Id, start, f.service.Id, extra)
}

// book creates an appointment at start (90 minutes) and returns it.
func (f bookingFixture) book(t *testing.T, start string) gen.Appointment {
	t.Helper()
	return decodeInto[gen.Appointment](t, f.send(http.MethodPost, appointmentsPath, f.bookBody(start, "")), http.StatusCreated)
}

func appointmentPath(id uuid.UUID) string { return appointmentsPath + "/" + id.String() }

func statusPath(id uuid.UUID) string { return appointmentPath(id) + "/status" }

// overlapBody decodes an overlap 409 and returns its title and appointments.
func overlapBody(t *testing.T, rec *httptest.ResponseRecorder) (string, []json.RawMessage) {
	t.Helper()
	assertProblem(t, rec, http.StatusConflict, ":overlap")
	var body struct {
		Title        string            `json:"title"`
		Appointments []json.RawMessage `json:"appointments"`
		Errors       json.RawMessage   `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body.Errors != nil {
		t.Fatalf("overlap body has errors %s, want none", body.Errors)
	}
	return body.Title, body.Appointments
}

func TestCreateAppointmentOK(t *testing.T) {
	f := newBookingFixture(t)
	rec := f.send(http.MethodPost, appointmentsPath, f.bookBody("2026-11-10T10:00:00Z", ""))
	got := decodeInto[gen.Appointment](t, rec, http.StatusCreated)
	assertKeys(t, rec.Body.Bytes(), appointmentKeys...)
	if got.Done || got.ClientName != "Lucía Rojas" || got.Status != gen.AppointmentStatusScheduled || got.TotalCents != 2500 {
		t.Fatalf("appointment = %+v, want not done, Lucía Rojas, scheduled, total 2500", got)
	}
	if got.EndsAt.Sub(got.StartsAt).Minutes() != 90 || len(got.Services) != 1 {
		t.Fatalf("ends_at %v, %d services, want 90 minutes and one line", got.EndsAt.Sub(got.StartsAt), len(got.Services))
	}
	var raw struct {
		Services []json.RawMessage `json:"services"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	assertKeys(t, raw.Services[0], "duration_min", "name", "price_cents", "service_id")
}

func TestCreateAppointmentMissingStartsAtIs422(t *testing.T) {
	f := newBookingFixture(t)
	body := fmt.Sprintf(`{"client_id":%q,"service_ids":[%q]}`, f.client.Id, f.service.Id)
	errs := fieldErrors(t, f.send(http.MethodPost, appointmentsPath, body))
	if len(errs) != 1 || errs[0].Field != "starts_at" {
		t.Fatalf("errors = %+v, want one on starts_at", errs)
	}
}

func TestPatchAppointmentZeroTimeIs422(t *testing.T) {
	f := newBookingFixture(t)
	made := f.book(t, "2026-11-10T10:00:00Z")
	for _, field := range []string{"starts_at", "ends_at"} {
		body := fmt.Sprintf(`{%q:"0001-01-01T00:00:00Z"}`, field)
		errs := fieldErrors(t, f.send(http.MethodPatch, appointmentPath(made.Id), body))
		if len(errs) != 1 || errs[0].Field != field {
			t.Fatalf("errors = %+v, want one on %s", errs, field)
		}
	}
}

func TestCreateAppointmentOverlapIs409WithAppointments(t *testing.T) {
	f := newBookingFixture(t)
	first := f.book(t, "2026-11-10T10:00:00Z")
	title, found := overlapBody(t, f.send(http.MethodPost, appointmentsPath, f.bookBody("2026-11-10T11:00:00Z", "")))
	if title != "Ya hay una cita a esa hora" || len(found) != 1 {
		t.Fatalf("title %q, %d appointments, want the title and one appointment", title, len(found))
	}
	assertKeys(t, found[0], appointmentKeys...)
	var listed gen.Appointment
	if err := json.Unmarshal(found[0], &listed); err != nil || listed.Id != first.Id {
		t.Fatalf("appointments[0] = %+v, %v, want the existing one", listed, err)
	}
}

func TestCreateAppointmentAllowOverlapIs201(t *testing.T) {
	f := newBookingFixture(t)
	f.book(t, "2026-11-10T10:00:00Z")
	rec := f.send(http.MethodPost, appointmentsPath, f.bookBody("2026-11-10T11:00:00Z", `,"allow_overlap":true`))
	decodeInto[gen.Appointment](t, rec, http.StatusCreated)
}

func TestCreateAppointmentArchivedClientIs422(t *testing.T) {
	f := newBookingFixture(t)
	f.send(http.MethodPatch, clientPath(f.client.Id), `{"archived":true}`)
	errs := fieldErrors(t, f.send(http.MethodPost, appointmentsPath, f.bookBody("2026-11-10T10:00:00Z", "")))
	if len(errs) != 1 || errs[0].Field != "client_id" {
		t.Fatalf("errors = %+v, want one on client_id", errs)
	}
}

func TestCreateAppointmentUnknownServiceIs422Field(t *testing.T) {
	f := newBookingFixture(t)
	body := fmt.Sprintf(`{"client_id":%q,"starts_at":"2026-11-10T10:00:00Z","service_ids":[%q]}`, f.client.Id, uuid.New())
	errs := fieldErrors(t, f.send(http.MethodPost, appointmentsPath, body))
	if len(errs) != 1 || errs[0].Field != "service_ids" {
		t.Fatalf("errors = %+v, want one on service_ids", errs)
	}
}

func TestGetAppointmentOK(t *testing.T) {
	f := newBookingFixture(t)
	made := f.book(t, "2026-11-10T10:00:00Z")
	rec := f.send(http.MethodGet, appointmentPath(made.Id), "")
	got := decodeInto[gen.Appointment](t, rec, http.StatusOK)
	assertKeys(t, rec.Body.Bytes(), appointmentKeys...)
	if got.Id != made.Id {
		t.Fatalf("id = %s, want %s", got.Id, made.Id)
	}
}

func TestGetAppointmentBadUUIDIs422(t *testing.T) {
	f := newBookingFixture(t)
	assertProblem(t, f.send(http.MethodGet, appointmentsPath+"/not-a-uuid", ""), http.StatusUnprocessableEntity, ":validation")
}

func TestGetAppointmentUnknownIs404(t *testing.T) {
	f := newBookingFixture(t)
	assertProblem(t, f.send(http.MethodGet, appointmentPath(uuid.New()), ""), http.StatusNotFound, ":not-found")
}

func TestUpdateAppointmentOverlapIs409(t *testing.T) {
	f := newBookingFixture(t)
	f.book(t, "2026-11-10T10:00:00Z")
	later := f.book(t, "2026-11-10T14:00:00Z")
	_, found := overlapBody(t, f.send(http.MethodPatch, appointmentPath(later.Id), `{"starts_at":"2026-11-10T11:00:00Z"}`))
	if len(found) != 1 {
		t.Fatalf("%d appointments in the 409, want 1", len(found))
	}
	moved := f.send(http.MethodPatch, appointmentPath(later.Id), `{"starts_at":"2026-11-10T11:00:00Z","allow_overlap":true}`)
	decodeInto[gen.Appointment](t, moved, http.StatusOK)
}

func TestPatchAppointmentClientIdIs422(t *testing.T) {
	f := newBookingFixture(t)
	made := f.book(t, "2026-11-10T10:00:00Z")
	body := fmt.Sprintf(`{"client_id":%q}`, uuid.New())
	assertProblem(t, f.send(http.MethodPatch, appointmentPath(made.Id), body), http.StatusUnprocessableEntity, ":validation")
}

func TestPatchAppointmentNullStartsAtIs422(t *testing.T) {
	f := newBookingFixture(t)
	made := f.book(t, "2026-11-10T10:00:00Z")
	errs := fieldErrors(t, f.send(http.MethodPatch, appointmentPath(made.Id), `{"starts_at":null}`))
	if len(errs) != 1 || errs[0].Field != "starts_at" {
		t.Fatalf("errors = %+v, want one on starts_at", errs)
	}
}

func TestPatchAppointmentNullPaidClears(t *testing.T) {
	f := newBookingFixture(t)
	made := f.book(t, "2026-11-10T10:00:00Z")
	paid := f.send(http.MethodPatch, appointmentPath(made.Id), `{"paid_cents":1000,"notes":"anticipo"}`)
	decodeInto[gen.Appointment](t, paid, http.StatusOK)
	if !strings.Contains(paid.Body.String(), `"paid_cents":1000`) {
		t.Fatalf("body %q lacks the paid amount", paid.Body.String())
	}
	cleared := f.send(http.MethodPatch, appointmentPath(made.Id), `{"paid_cents":null,"notes":null}`)
	decodeInto[gen.Appointment](t, cleared, http.StatusOK)
	for _, want := range []string{`"paid_cents":null`, `"notes":null`} {
		if !strings.Contains(cleared.Body.String(), want) {
			t.Fatalf("body %q lacks %s", cleared.Body.String(), want)
		}
	}
}

func TestPatchAppointmentUnknownIs404(t *testing.T) {
	f := newBookingFixture(t)
	assertProblem(t, f.send(http.MethodPatch, appointmentPath(uuid.New()), `{"notes":"x"}`), http.StatusNotFound, ":not-found")
}

func TestSetStatusOK(t *testing.T) {
	f := newBookingFixture(t)
	made := f.book(t, "2026-11-10T10:00:00Z")
	got := decodeInto[gen.Appointment](t, f.send(http.MethodPost, statusPath(made.Id), `{"status":"confirmed"}`), http.StatusOK)
	if got.Status != gen.AppointmentStatusConfirmed {
		t.Fatalf("status = %s, want confirmed", got.Status)
	}
}

func TestSetStatusReactivateOverlapIs409(t *testing.T) {
	f := newBookingFixture(t)
	first := f.book(t, "2026-11-10T10:00:00Z")
	decodeInto[gen.Appointment](t, f.send(http.MethodPost, statusPath(first.Id), `{"status":"cancelled"}`), http.StatusOK)
	f.book(t, "2026-11-10T10:00:00Z")
	_, found := overlapBody(t, f.send(http.MethodPost, statusPath(first.Id), `{"status":"scheduled"}`))
	if len(found) != 1 {
		t.Fatalf("%d appointments in the 409, want 1", len(found))
	}
	rec := f.send(http.MethodPost, statusPath(first.Id), `{"status":"scheduled","allow_overlap":true}`)
	if got := decodeInto[gen.Appointment](t, rec, http.StatusOK); got.Status != gen.AppointmentStatusScheduled {
		t.Fatalf("status = %s, want scheduled", got.Status)
	}
}

func TestSetStatusInvalidIs409(t *testing.T) {
	f := newBookingFixture(t)
	made := f.book(t, "2026-11-10T10:00:00Z")
	f.send(http.MethodPost, statusPath(made.Id), `{"status":"cancelled"}`)
	rec := f.send(http.MethodPost, statusPath(made.Id), `{"status":"confirmed"}`)
	assertProblem(t, rec, http.StatusConflict, ":invalid-transition")
	assertKeys(t, rec.Body.Bytes(), "status", "title", "type")
}

func TestSetStatusUnknownValueIs422(t *testing.T) {
	f := newBookingFixture(t)
	made := f.book(t, "2026-11-10T10:00:00Z")
	errs := fieldErrors(t, f.send(http.MethodPost, statusPath(made.Id), `{"status":"done"}`))
	if len(errs) != 1 || errs[0].Field != "status" {
		t.Fatalf("errors = %+v, want one on status", errs)
	}
}

func TestSetStatusUnknownAppointmentIs404(t *testing.T) {
	f := newBookingFixture(t)
	assertProblem(t, f.send(http.MethodPost, statusPath(uuid.New()), `{"status":"confirmed"}`), http.StatusNotFound, ":not-found")
}

func TestListClientAppointmentsNewestFirst(t *testing.T) {
	f := newBookingFixture(t)
	older := f.book(t, "2026-11-10T10:00:00Z")
	newer := f.book(t, "2026-11-12T10:00:00Z")
	list := decodeInto[gen.AppointmentList](t, f.send(http.MethodGet, clientPath(f.client.Id)+"/appointments", ""), http.StatusOK)
	if len(list.Items) != 2 || list.Items[0].Id != newer.Id || list.Items[1].Id != older.Id {
		t.Fatalf("items = %+v, want newer then older", list.Items)
	}
}

func TestListClientAppointmentsEmptyIsArray(t *testing.T) {
	f := newBookingFixture(t)
	rec := f.send(http.MethodGet, clientPath(f.client.Id)+"/appointments", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"items":[]}` {
		t.Fatalf("got %d %q, want 200 {\"items\":[]}", rec.Code, rec.Body.String())
	}
}

func TestListClientAppointmentsUnknownIs404(t *testing.T) {
	f := newBookingFixture(t)
	assertProblem(t, f.send(http.MethodGet, clientPath(uuid.New())+"/appointments", ""), http.StatusNotFound, ":not-found")
}

func TestAppointmentRoutesNeedSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	id := uuid.NewString()
	for _, rt := range []struct{ method, target, body string }{
		{http.MethodGet, appointmentsPath + "?from=2026-11-01&to=2026-11-02", ""},
		{http.MethodPost, appointmentsPath, `{}`},
		{http.MethodGet, appointmentsPath + "/" + id, ""},
		{http.MethodPatch, appointmentsPath + "/" + id, `{"notes":"x"}`},
		{http.MethodPost, appointmentsPath + "/" + id + "/status", `{"status":"confirmed"}`},
		{http.MethodGet, clientsPath + "/" + id + "/appointments", ""},
	} {
		t.Run(rt.method+" "+rt.target, func(t *testing.T) {
			assertUnauthorized(t, send(app.handler, rt.method, rt.target, testOrigin, rt.body, nil))
		})
	}
}
