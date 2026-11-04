//go:build db

package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

const rulesPath = "/api/v1/reminder-rules"

func ruleTemplate(t *testing.T, app testApp, c *http.Cookie) gen.Template {
	t.Helper()
	return createTemplate(t, app, c, templateBody)
}

func beforeRuleBody(tpl gen.Template) string {
	return `{"kind":"before_appointment","offset_days":1,"template_id":"` + tpl.Id.String() + `"}`
}

func createRule(t *testing.T, app testApp, c *http.Cookie, body string) gen.Rule {
	t.Helper()
	return decodeInto[gen.Rule](t, sendAs(app, c, http.MethodPost, rulesPath, body), http.StatusCreated)
}

func TestCreateRuleOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := ruleTemplate(t, app, c)
	rec := sendAs(app, c, http.MethodPost, rulesPath, beforeRuleBody(tpl))
	got := decodeInto[gen.Rule](t, rec, http.StatusCreated)
	assertKeys(t, rec.Body.Bytes(), "applies_from", "archived", "id", "kind", "offset_days", "return_service_id", "service_id", "template_id")
	if got.Kind != gen.RuleKind("before_appointment") || got.OffsetDays != 1 || got.TemplateId != tpl.Id || got.Archived {
		t.Fatalf("rule = %+v, want a before rule, offset 1, the template, active", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"service_id":null`) || !strings.Contains(body, `"return_service_id":null`) {
		t.Fatalf("body %q lacks null service_id and return_service_id", body)
	}
}

func TestCreateComeBackRuleWithReturnService(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := ruleTemplate(t, app, c)
	own := createService(t, app, c, serviceBody)
	back := createService(t, app, c, `{"name":"Relleno","duration_min":45,"color":"#112233"}`)
	got := createRule(t, app, c, `{"kind":"come_back","offset_days":21,"template_id":"`+tpl.Id.String()+
		`","service_id":"`+own.Id.String()+`","return_service_id":"`+back.Id.String()+`"}`)
	if v, err := got.ServiceId.Get(); err != nil || v != own.Id {
		t.Fatalf("service_id = %v, %v, want %v", v, err, own.Id)
	}
	if v, err := got.ReturnServiceId.Get(); err != nil || v != back.Id {
		t.Fatalf("return_service_id = %v, %v, want %v", v, err, back.Id)
	}
}

func TestCreateRuleBadKindIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := ruleTemplate(t, app, c)
	rec := sendAs(app, c, http.MethodPost, rulesPath, `{"kind":"owner_task","offset_days":1,"template_id":"`+tpl.Id.String()+`"}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "kind" {
		t.Fatalf("errors = %+v, want one on kind", errs)
	}
}

func TestCreateRuleArchivedTemplateIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := ruleTemplate(t, app, c)
	sendAs(app, c, http.MethodPatch, templatesPath+"/"+tpl.Id.String(), `{"archived":true}`)
	rec := sendAs(app, c, http.MethodPost, rulesPath, beforeRuleBody(tpl))
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "template_id" {
		t.Fatalf("errors = %+v, want one on template_id", errs)
	}
}

func TestPatchRuleKindIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rule := createRule(t, app, c, beforeRuleBody(ruleTemplate(t, app, c)))
	rec := sendAs(app, c, http.MethodPatch, rulesPath+"/"+rule.Id.String(), `{"kind":"come_back"}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPatchRuleNullOffsetIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rule := createRule(t, app, c, beforeRuleBody(ruleTemplate(t, app, c)))
	rec := sendAs(app, c, http.MethodPatch, rulesPath+"/"+rule.Id.String(), `{"offset_days":null}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "offset_days" {
		t.Fatalf("errors = %+v, want one on offset_days", errs)
	}
}

func TestPatchRuleNullReturnServiceClears(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := ruleTemplate(t, app, c)
	own := createService(t, app, c, serviceBody)
	back := createService(t, app, c, `{"name":"Relleno","duration_min":45,"color":"#112233"}`)
	rule := createRule(t, app, c, `{"kind":"come_back","offset_days":21,"template_id":"`+tpl.Id.String()+
		`","service_id":"`+own.Id.String()+`","return_service_id":"`+back.Id.String()+`"}`)
	rec := sendAs(app, c, http.MethodPatch, rulesPath+"/"+rule.Id.String(), `{"return_service_id":null}`)
	got := decodeInto[gen.Rule](t, rec, http.StatusOK)
	if !got.ReturnServiceId.IsNull() || !strings.Contains(rec.Body.String(), `"return_service_id":null`) {
		t.Fatalf("rule = %+v, body %q, want return_service_id null", got, rec.Body.String())
	}
}

func TestPatchRuleBadUUIDIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPatch, rulesPath+"/not-a-uuid", `{"archived":true}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPatchRuleUnknownIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPatch, rulesPath+"/"+uuid.NewString(), `{"archived":true}`)
	assertProblem(t, rec, http.StatusNotFound, ":not-found")
}

func TestRuleRoutesNeedSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	for _, rt := range []struct{ method, target, body string }{
		{http.MethodPost, rulesPath, `{"kind":"before_appointment","offset_days":1,"template_id":"` + uuid.NewString() + `"}`},
		{http.MethodPatch, rulesPath + "/" + uuid.NewString(), `{"archived":true}`},
	} {
		t.Run(rt.method+" "+rt.target, func(t *testing.T) {
			assertUnauthorized(t, send(app.handler, rt.method, rt.target, testOrigin, rt.body, nil))
		})
	}
}
