//go:build db

package httpapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// The rule preview and the client's next message (reminders-and-replies design, "Look-ahead pass";
// item 44). The module tests hold the computation; these hold the wire.

func TestPreviewRuleAnswersCountAndNullNext(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := ruleTemplate(t, app, c)
	rec := sendAs(app, c, http.MethodPost, rulesPath+"/preview", `{"offset_days":30,"template_id":"`+tpl.Id.String()+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%q)", rec.Code, rec.Body.String())
	}
	assertKeys(t, rec.Body.Bytes(), "next", "today_count")
	if got := rec.Body.String(); got != `{"next":null,"today_count":0}`+"\n" {
		t.Fatalf("body = %q, want no clients and a null next", got)
	}
}

func TestPreviewRuleOfABeforeRuleIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := ruleTemplate(t, app, c)
	before := createRule(t, app, c, beforeRuleBody(tpl))
	rec := sendAs(app, c, http.MethodPost, rulesPath+"/preview",
		`{"rule_id":"`+before.Id.String()+`","offset_days":30,"template_id":"`+tpl.Id.String()+`"}`)
	if errs := fieldErrors(t, rec); len(errs) != 1 || errs[0].Field != "rule_id" {
		t.Fatalf("errors = %+v, want one on rule_id", errs)
	}
}

func TestPreviewRuleUnknownRuleIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := ruleTemplate(t, app, c)
	rec := sendAs(app, c, http.MethodPost, rulesPath+"/preview",
		`{"rule_id":"`+uuid.NewString()+`","offset_days":30,"template_id":"`+tpl.Id.String()+`"}`)
	assertProblem(t, rec, http.StatusNotFound, ":not-found")
}

func TestNextReminderUnknownClientIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodGet, clientsPath+"/"+uuid.NewString()+"/next-reminder", "")
	assertProblem(t, rec, http.StatusNotFound, ":not-found")
}

func TestNextReminderAnswersTheBeforeReminder(t *testing.T) {
	f := newBookingFixture(t)
	createRule(t, f.app, f.cookie, beforeRuleBody(ruleTemplate(t, f.app, f.cookie)))
	f.book(t, "2026-04-10T10:00:00Z") // testStart is 2 Mar 2026
	rec := f.send(http.MethodGet, clientsPath+"/"+f.client.Id.String()+"/next-reminder", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%q)", rec.Code, rec.Body.String())
	}
	assertKeys(t, rec.Body.Bytes(), "due_date", "general", "source_services")
	if got := rec.Body.String(); got != `{"due_date":"2026-04-09","general":false,"source_services":["Acrílico"]}`+"\n" {
		t.Fatalf("body = %q, want the before reminder on 9 Apr", got)
	}
}
