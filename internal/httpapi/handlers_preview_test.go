//go:build db

package httpapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const previewPath = "/api/v1/templates/preview"

func TestPreviewTemplateOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, previewPath, `{"body":"Hola {{client.first_name}}"}`)
	got := decodeInto[gen.TemplatePreview](t, rec, http.StatusOK)
	assertKeys(t, rec.Body.Bytes(), "text", "unknown_vars")
	if got.Text != "Hola Ana" || got.UnknownVars == nil || len(got.UnknownVars) != 0 {
		t.Fatalf("preview = %+v; want the sample greeting and an empty unknown_vars", got)
	}
}

func TestPreviewTemplateListsUnknownVars(t *testing.T) {
	app, _, c := signedInOwner(t)
	got := decodeInto[gen.TemplatePreview](t, sendAs(app, c, http.MethodPost, previewPath, `{"body":"{{client.zz}} y {{client.zz}}"}`), http.StatusOK)
	if len(got.UnknownVars) != 1 || got.UnknownVars[0] != "client.zz" {
		t.Fatalf("unknown_vars = %v; want [client.zz]", got.UnknownVars)
	}
}

func TestPreviewTemplateUnknownClientIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, previewPath, `{"body":"Hola","client_id":"`+uuid.NewString()+`"}`)
	errs := fieldErrors(t, rec)
	want := problem.FieldError{Field: "client_id", Message: problem.MsgClientUnavailable}
	if len(errs) != 1 || errs[0] != want {
		t.Fatalf("errors = %+v, want [%+v]", errs, want)
	}
}

func TestPreviewTemplateUnknownFieldIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, previewPath, `{"body":"Hola","extra":1}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPreviewTemplateBadClientIdIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, previewPath, `{"body":"Hola","client_id":"nope"}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPreviewTemplateNeedsSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, send(app.handler, http.MethodPost, previewPath, testOrigin, `{"body":"Hola"}`, nil))
}
