//go:build db

package httpapi

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const templatesPath = "/api/v1/templates"

const templateBody = `{"name":"Recordatorio","body":"Hola {{client.first_name}}, te espero el {{appointment.date}}"}`

func createTemplate(t *testing.T, app testApp, c *http.Cookie, body string) gen.Template {
	t.Helper()
	return decodeInto[gen.Template](t, sendAs(app, c, http.MethodPost, templatesPath, body), http.StatusCreated)
}

func TestCreateTemplateOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, templatesPath, templateBody)
	got := decodeInto[gen.Template](t, rec, http.StatusCreated)
	assertKeys(t, rec.Body.Bytes(), "archived", "body", "id", "name")
	if got.Name != "Recordatorio" || got.Body != "Hola {{client.first_name}}, te espero el {{appointment.date}}" || got.Archived {
		t.Fatalf("template = %+v, want the stored name and body, active", got)
	}
}

func TestCreateTemplateUnknownVarIs422Message(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, templatesPath, `{"name":"X","body":"Hola {{client.bebida_favorita}}"}`)
	errs := fieldErrors(t, rec)
	want := problem.FieldError{Field: "body", Message: "La variable 'bebida_favorita' no existe"}
	if len(errs) != 1 || errs[0] != want {
		t.Fatalf("errors = %+v, want [%+v]", errs, want)
	}
}

func TestCreateTemplateUnclosedIs422Position(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, templatesPath, `{"name":"X","body":"Hola, Lucía {{client.name"}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "body" || errs[0].Message != fmt.Sprintf(problem.MsgTemplateUnclosed, 13) {
		t.Fatalf("errors = %+v, want one body error at position 13", errs)
	}
}

func TestPatchTemplateArchive(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := createTemplate(t, app, c, templateBody)
	got := decodeInto[gen.Template](t, sendAs(app, c, http.MethodPatch, templatesPath+"/"+tpl.Id.String(), `{"archived":true}`), http.StatusOK)
	if !got.Archived || got.Id != tpl.Id || got.Body != tpl.Body || got.Name != tpl.Name {
		t.Fatalf("template = %+v, want archived with name and body unchanged", got)
	}
}

func TestPatchTemplateNullBodyIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	tpl := createTemplate(t, app, c, templateBody)
	rec := sendAs(app, c, http.MethodPatch, templatesPath+"/"+tpl.Id.String(), `{"body":null}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "body" {
		t.Fatalf("errors = %+v, want one on body", errs)
	}
}

func TestPatchTemplateBadUUIDIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPatch, templatesPath+"/not-a-uuid", `{"archived":true}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPatchTemplateUnknownIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPatch, templatesPath+"/"+uuid.NewString(), `{"archived":true}`)
	assertProblem(t, rec, http.StatusNotFound, ":not-found")
}

func TestTemplateRoutesNeedSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	for _, rt := range []struct{ method, target, body string }{
		{http.MethodPost, templatesPath, templateBody},
		{http.MethodPatch, templatesPath + "/" + uuid.NewString(), `{"archived":true}`},
	} {
		t.Run(rt.method+" "+rt.target, func(t *testing.T) {
			assertUnauthorized(t, send(app.handler, rt.method, rt.target, testOrigin, rt.body, nil))
		})
	}
}
