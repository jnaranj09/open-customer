//go:build db

package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

const (
	reportPath       = "/api/v1/status"
	clientErrorsPath = "/api/v1/client-errors"
)

const clientErrorBody = `{"message":"boom","app_version":"1.2.3","route":"/hoy"}`

func TestGetStatusOK(t *testing.T) {
	app, _, c := signedInOwner(t)

	rec := sendAs(app, c, http.MethodGet, reportPath, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%q)", rec.Code, rec.Body.String())
	}
	assertKeys(t, rec.Body.Bytes(), "version", "migration_version", "heartbeat_age_s", "backup", "media_free_bytes", "push")
	var top map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &top); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, top["backup"], "checked_at", "last_dump_at", "last_copy_at", "last_error", "red")
	assertKeys(t, top["push"], "subscriptions", "last_push", "red")
	var backup struct{ Red bool }
	if err := json.Unmarshal(top["backup"], &backup); err != nil || !backup.Red {
		t.Errorf("backup.red = %v (%v), want true: the test app has no backup file", backup.Red, err)
	}
	if string(top["heartbeat_age_s"]) != "null" {
		t.Errorf("heartbeat_age_s = %s, want null before the first tick", top["heartbeat_age_s"])
	}
}

func TestGetStatusNeedsSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, send(app.handler, http.MethodGet, reportPath, testOrigin, "", nil))
}

func TestPostClientErrorIs204(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, clientErrorsPath, `{"message":"boom","stack":"at x","app_version":"1.2.3","route":"/hoy"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d (%q), want 204", rec.Code, rec.Body.String())
	}
}

func TestPostClientErrorNeedsSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, send(app.handler, http.MethodPost, clientErrorsPath, testOrigin, clientErrorBody, nil))
}

func TestPostClientErrorEleventhIs429(t *testing.T) {
	app, _, c := signedInOwner(t)
	for i := range 10 {
		if rec := sendAs(app, c, http.MethodPost, clientErrorsPath, clientErrorBody); rec.Code != http.StatusNoContent {
			t.Fatalf("call %d status = %d", i+1, rec.Code)
		}
	}
	assertRateLimited(t, sendAs(app, c, http.MethodPost, clientErrorsPath, clientErrorBody))
}

func TestPostClientErrorUnknownFieldIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, clientErrorsPath, `{"message":"boom","app_version":"1","route":"/","extra":1}`)
	fieldErrors(t, rec)
}

func TestPostClientErrorEmptyMessageIs422OnTheField(t *testing.T) {
	app, _, c := signedInOwner(t)
	errs := fieldErrors(t, sendAs(app, c, http.MethodPost, clientErrorsPath, `{"message":"","app_version":"1","route":"/"}`))
	if len(errs) != 1 || errs[0].Field != "message" {
		t.Fatalf("errors = %+v, want one on message", errs)
	}
}
