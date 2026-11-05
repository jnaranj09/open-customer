//go:build db

package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const bootstrapPath = "/api/v1/bootstrap"

// bootstrapKeys are the keys of a Bootstrap.
var bootstrapKeys = []string{
	"appointments", "attributes", "backup_red", "clients", "due", "rules", "server_time", "services",
	"settings", "template_vars", "templates",
}

// archive archives the record at path as the owner c.
func archive(t *testing.T, app testApp, c *http.Cookie, path string) {
	t.Helper()
	if rec := sendAs(app, c, http.MethodPatch, path, `{"archived":true}`); rec.Code != http.StatusOK {
		t.Fatalf("archive %s: status %d (%q)", path, rec.Code, rec.Body.String())
	}
}

func bootstrapURL(from, to string) string {
	return bootstrapPath + "?from=" + url.QueryEscape(from) + "&to=" + url.QueryEscape(to)
}

func TestBootstrapOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	// The test clock reads 2026-03-02 10:00 UTC.
	oldAttr := createSelectAttribute(t, app, c)
	archive(t, app, c, attributesPath+"/"+oldAttr.Id.String())
	oldService := createService(t, app, c, `{"name":"Retoque","duration_min":30,"color":"#AA3366"}`)
	archive(t, app, c, servicesPath+"/"+oldService.Id.String())
	service := createService(t, app, c, `{"name":"Acrílico","duration_min":60,"color":"#336699"}`)
	oldTemplate := createTemplate(t, app, c, `{"name":"Antigua","body":"Hola"}`)
	archive(t, app, c, templatesPath+"/"+oldTemplate.Id.String())
	template := createTemplate(t, app, c, `{"name":"Recordatorio","body":"Hola {{client.first_name}}, te esperamos"}`)
	oldRule := createRule(t, app, c, fmt.Sprintf(`{"kind":"before_appointment","offset_days":3,"include_overdue":true,"template_id":%q}`, template.Id))
	archive(t, app, c, rulesPath+"/"+oldRule.Id.String())
	createRule(t, app, c, fmt.Sprintf(`{"kind":"before_appointment","offset_days":1,"include_overdue":true,"template_id":%q}`, template.Id))
	ana := createClient(t, app, c, `{"name":"Ana Rojas","phone":"612 345 678"}`)
	gone := createClient(t, app, c, `{"name":"Marta Solano"}`)
	archive(t, app, c, clientPath(gone.Id))
	inWindow := bookAt(t, app, c, ana, service, "2026-03-03T10:00:00+01:00")
	bookAt(t, app, c, ana, service, "2026-09-03T10:00:00+02:00")

	rec := sendAs(app, c, http.MethodGet, bootstrapURL("2026-03-01", "2026-04-01"), "")
	got := decodeInto[gen.Bootstrap](t, rec, http.StatusOK)

	assertKeys(t, rec.Body.Bytes(), bootstrapKeys...)
	if !got.ServerTime.Equal(testStart) {
		t.Errorf("server_time = %v, want %v", got.ServerTime, testStart)
	}
	if len(got.Attributes) != 1 || got.Attributes[0].Id != oldAttr.Id || !got.Attributes[0].Archived {
		t.Errorf("attributes = %+v, want the archived attribute", got.Attributes)
	}
	if len(got.Services) != 2 || !archivedService(got.Services, oldService.Id.String()) {
		t.Errorf("services = %+v, want both, the old one archived", got.Services)
	}
	if len(got.Templates) != 2 || !archivedTemplate(got.Templates, oldTemplate.Id.String()) {
		t.Errorf("templates = %+v, want both, the old one archived", got.Templates)
	}
	if len(got.Rules) != 2 || !archivedRule(got.Rules, oldRule.Id.String()) {
		t.Errorf("rules = %+v, want both, the old one archived", got.Rules)
	}
	if len(got.Clients) != 1 || got.Clients[0].Id != ana.Id {
		t.Errorf("clients = %+v, want only Ana Rojas", got.Clients)
	}
	if len(got.Appointments) != 1 || got.Appointments[0].Id != inWindow.Id {
		t.Errorf("appointments = %+v, want only the one in the window", got.Appointments)
	}
	if len(got.Due) != 1 || got.Due[0].AppointmentId != inWindow.Id || got.Due[0].Kind != "before_appointment" {
		t.Errorf("due = %+v, want the before-appointment item", got.Due)
	}
	if len(got.TemplateVars) == 0 || got.TemplateVars[0] != (gen.TemplateVar{Ns: "client", Key: "name", Label: "Nombre"}) {
		t.Errorf("template_vars = %+v, want client.name first, labelled Nombre", got.TemplateVars)
	}
	if got.Settings.Timezone == "" || !got.BackupRed {
		t.Errorf("settings = %+v, backup_red = %v; want the settings and a red backup (the test app has no backup file)", got.Settings, got.BackupRed)
	}
}

func archivedService(list []gen.Service, id string) bool {
	for _, v := range list {
		if v.Id.String() == id {
			return v.Archived
		}
	}
	return false
}

func archivedTemplate(list []gen.Template, id string) bool {
	for _, v := range list {
		if v.Id.String() == id {
			return v.Archived
		}
	}
	return false
}

func archivedRule(list []gen.Rule, id string) bool {
	for _, v := range list {
		if v.Id.String() == id {
			return v.Archived
		}
	}
	return false
}

func TestBootstrapEmptyListsAreArrays(t *testing.T) {
	app, _, c := signedInOwner(t)

	rec := sendAs(app, c, http.MethodGet, bootstrapURL("2026-03-01", "2026-04-01"), "")

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%q)", rec.Code, body)
	}
	for _, key := range []string{"attributes", "services", "templates", "rules", "clients", "appointments", "due"} {
		if !strings.Contains(body, `"`+key+`":[]`) {
			t.Errorf("body lacks %q: [] in %q", key, body)
		}
	}
	if strings.Contains(body, `"template_vars":null`) || strings.Contains(body, `"template_vars":[]`) {
		t.Errorf("template_vars = null or empty in %q; want the fixed variables", body)
	}
}

func TestBootstrapAt200DaysIs200(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodGet, bootstrapURL("2026-01-01", "2026-07-20"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%q), want 200 for exactly 200 days", rec.Code, rec.Body.String())
	}
}

func TestBootstrapOver200DaysIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	errs := fieldErrors(t, sendAs(app, c, http.MethodGet, bootstrapURL("2026-01-01", "2026-07-21"), ""))
	if len(errs) != 1 || errs[0].Field != "to" || errs[0].Message != problem.MsgBootstrapRangeInvalid {
		t.Fatalf("errors = %+v, want to: %q", errs, problem.MsgBootstrapRangeInvalid)
	}
}

func TestBootstrapToBeforeFromIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	errs := fieldErrors(t, sendAs(app, c, http.MethodGet, bootstrapURL("2026-03-02", "2026-03-01"), ""))
	if len(errs) != 1 || errs[0].Field != "to" || errs[0].Message != problem.MsgBootstrapRangeInvalid {
		t.Fatalf("errors = %+v, want to: %q", errs, problem.MsgBootstrapRangeInvalid)
	}
}

func TestBootstrapBadDateIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	assertProblem(t, sendAs(app, c, http.MethodGet, bootstrapURL("ayer", "2026-03-01"), ""), http.StatusUnprocessableEntity, ":validation")
}

func TestBootstrapNeedsSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, send(app.handler, http.MethodGet, bootstrapURL("2026-03-01", "2026-04-01"), testOrigin, "", nil))
}
