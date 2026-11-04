//go:build db

package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const logPath = "/api/v1/reminders/log"

// reminderFixture is a signed-in app with a client ("Lucía Rojas", with a phone), a service and a
// "before" rule one day ahead. The fake clock is at testStart, 2026-03-02 10:00 UTC.
type reminderFixture struct {
	app     testApp
	cookie  *http.Cookie
	client  gen.Client
	service gen.Service
	rule    gen.Rule
}

func newReminderFixture(t *testing.T) reminderFixture {
	t.Helper()
	app, _, c := signedInOwner(t)
	return reminderFixture{
		app: app, cookie: c,
		client:  createClient(t, app, c, clientBody),
		service: createService(t, app, c, serviceBody),
		rule:    createRule(t, app, c, beforeRuleBody(ruleTemplate(t, app, c))),
	}
}

func (f reminderFixture) send(method, target, body string) *httptest.ResponseRecorder {
	return sendAs(f.app, f.cookie, method, target, body)
}

// book books the fixture's client one day after testStart plus days.
func (f reminderFixture) book(t *testing.T, client gen.Client, days int) gen.Appointment {
	t.Helper()
	start := testStart.AddDate(0, 0, 1+days).Format(time.RFC3339)
	return bookAt(t, f.app, f.cookie, client, f.service, start)
}

// logBody is a POST /reminders/log body for a's "before" item, acted on at actedAt.
func (f reminderFixture) logBody(a gen.Appointment, action, actedAt string) string {
	return fmt.Sprintf(`{"rule_id":%q,"appointment_id":%q,"anchor_at":%q,"action":%q,"body":"Hola","phone_digits":"34612345678","acted_at":%q}`,
		f.rule.Id, a.Id, a.StartsAt.Format(time.RFC3339), action, actedAt)
}

func (f reminderFixture) record(t *testing.T, a gen.Appointment, actedAt string) gen.LogItem {
	t.Helper()
	return decodeInto[gen.LogItem](t, f.send(http.MethodPost, logPath, f.logBody(a, "sent", actedAt)), http.StatusCreated)
}

func TestPostReminderLogTwiceIs201Then200(t *testing.T) {
	f := newReminderFixture(t)
	a := f.book(t, f.client, 0)
	if due := decodeInto[gen.DueList](t, f.send(http.MethodGet, duePath, ""), http.StatusOK); len(due.Items) != 1 {
		t.Fatalf("due before logging = %+v; want the one item", due.Items)
	}
	body := f.logBody(a, "sent", "2026-03-02T10:00:00Z")
	first := decodeInto[gen.LogItem](t, f.send(http.MethodPost, logPath, body), http.StatusCreated)
	second := decodeInto[gen.LogItem](t, f.send(http.MethodPost, logPath, body), http.StatusOK)
	if first.Id != second.Id || first.ClientName != "Lucía Rojas" || first.Action != "sent" {
		t.Fatalf("first = %+v, second = %+v; want the same row for Lucía Rojas", first, second)
	}
	if due := decodeInto[gen.DueList](t, f.send(http.MethodGet, duePath, ""), http.StatusOK); len(due.Items) != 0 {
		t.Fatalf("due after logging = %+v; want none", due.Items)
	}
}

func TestPostReminderLogReturnsTheLogItemKeys(t *testing.T) {
	f := newReminderFixture(t)
	rec := f.send(http.MethodPost, logPath, f.logBody(f.book(t, f.client, 0), "skipped", "2026-03-02T10:00:00Z"))
	assertKeys(t, rec.Body.Bytes(), "acted_at", "action", "anchor_at", "appointment_id", "body", "client_id", "client_name", "id", "kind", "phone_digits", "rule_id")
}

func TestPostReminderLogUnknownRuleIs422(t *testing.T) {
	f := newReminderFixture(t)
	body := fmt.Sprintf(`{"rule_id":%q,"appointment_id":%q,"anchor_at":%q,"action":"sent","acted_at":"2026-03-02T10:00:00Z"}`,
		uuid.New(), uuid.New(), testStart.Format(time.RFC3339))
	for _, e := range fieldErrors(t, f.send(http.MethodPost, logPath, body)) {
		if e.Field == "rule_id" && e.Message == problem.MsgRuleUnknown {
			return
		}
	}
	t.Fatal("no rule_id error with the unknown-rule message")
}

func TestPostReminderLogBadActionIs422(t *testing.T) {
	f := newReminderFixture(t)
	errs := fieldErrors(t, f.send(http.MethodPost, logPath, f.logBody(f.book(t, f.client, 0), "done", "2026-03-02T10:00:00Z")))
	if len(errs) != 1 || errs[0].Field != "action" {
		t.Fatalf("errors = %+v; want one on action", errs)
	}
}

func TestPostReminderLogUnknownFieldIs422(t *testing.T) {
	f := newReminderFixture(t)
	body := fmt.Sprintf(`{"rule_id":%q,"appointment_id":%q,"action":"sent","acted_at":"2026-03-02T10:00:00Z","extra":1}`, f.rule.Id, uuid.New())
	assertProblem(t, f.send(http.MethodPost, logPath, body), http.StatusUnprocessableEntity, ":validation")
}

func TestGetReminderLogByDate(t *testing.T) {
	f := newReminderFixture(t)
	older := f.record(t, f.book(t, f.client, 0), "2026-10-22T08:00:00Z")
	newer := f.record(t, f.book(t, f.client, 1), "2026-10-22T09:00:00Z")
	f.record(t, f.book(t, f.client, 2), "2026-10-21T21:00:00Z") // 23:00 on the 21st in Madrid
	got := decodeInto[gen.LogList](t, f.send(http.MethodGet, logPath+"?date=2026-10-22", ""), http.StatusOK).Items
	if len(got) != 2 || got[0].Id != newer.Id || got[1].Id != older.Id {
		t.Fatalf("items = %+v; want [%s %s] (newest first, the 21st left out)", got, newer.Id, older.Id)
	}
}

func TestGetReminderLogByClient(t *testing.T) {
	f := newReminderFixture(t)
	marta := createClient(t, f.app, f.cookie, `{"name":"Marta Solano"}`)
	mine := f.record(t, f.book(t, f.client, 0), "2026-03-02T10:00:00Z")
	f.record(t, f.book(t, marta, 1), "2026-03-02T11:00:00Z")
	got := decodeInto[gen.LogList](t, f.send(http.MethodGet, logPath+"?client_id="+f.client.Id.String(), ""), http.StatusOK).Items
	if len(got) != 1 || got[0].Id != mine.Id {
		t.Fatalf("items = %+v; want only %s", got, mine.Id)
	}
}

func TestGetReminderLogNeedsExactlyOneFilter(t *testing.T) {
	f := newReminderFixture(t)
	for name, query := range map[string]string{
		"no filter":    "",
		"both filters": "?date=2026-10-22&client_id=" + f.client.Id.String(),
	} {
		t.Run(name, func(t *testing.T) {
			errs := fieldErrors(t, f.send(http.MethodGet, logPath+query, ""))
			if len(errs) != 1 || errs[0].Field != "client_id" || errs[0].Message != problem.MsgLogFilterInvalid {
				t.Fatalf("errors = %+v; want one on client_id", errs)
			}
		})
	}
}

func TestGetReminderLogBadDateIs422(t *testing.T) {
	f := newReminderFixture(t)
	assertProblem(t, f.send(http.MethodGet, logPath+"?date=2026-13-01", ""), http.StatusUnprocessableEntity, ":validation")
}

func TestGetReminderLogUnknownClientIs404(t *testing.T) {
	f := newReminderFixture(t)
	assertProblem(t, f.send(http.MethodGet, logPath+"?client_id="+uuid.NewString(), ""), http.StatusNotFound, ":not-found")
}

func TestReminderRoutesNeedSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	for name, tc := range map[string]struct{ method, target, body string }{
		"due":      {http.MethodGet, duePath, ""},
		"log post": {http.MethodPost, logPath, `{}`},
		"log get":  {http.MethodGet, logPath + "?date=2026-10-22", ""},
	} {
		t.Run(name, func(t *testing.T) {
			assertUnauthorized(t, send(app.handler, tc.method, tc.target, testOrigin, tc.body, nil))
		})
	}
}
