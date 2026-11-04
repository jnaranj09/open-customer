//go:build db

package httpapi

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// sendAs sends body with method to target as the owner c, with the public Origin.
func sendAs(app testApp, c *http.Cookie, method, target, body string) *httptest.ResponseRecorder {
	return send(app.handler, method, target, testOrigin, body, withCookie(c))
}

// decodeInto asserts rec has status and decodes its JSON body into a T.
func decodeInto[T any](t *testing.T, rec *httptest.ResponseRecorder, status int) T {
	t.Helper()
	var v T
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, status, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("body is not the wanted JSON: %v (%q)", err, rec.Body.String())
	}
	return v
}

// assertKeys asserts the top-level keys of the JSON object raw are exactly want.
func assertKeys(t *testing.T, raw []byte, want ...string) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("body is not a JSON object: %v (%q)", err, raw)
	}
	got := slices.Sorted(maps.Keys(obj))
	if !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Fatalf("keys = %v, want %v", got, slices.Sorted(slices.Values(want)))
	}
}

// fieldErrors asserts rec is a 422 validation problem and returns its errors.
func fieldErrors(t *testing.T, rec *httptest.ResponseRecorder) []problem.FieldError {
	t.Helper()
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
	var body struct {
		Errors []problem.FieldError `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	return body.Errors
}
