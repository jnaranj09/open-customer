package problem

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, err error) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	Write(rec, httptest.NewRequest(http.MethodGet, "/api/v1/x", nil), err)
	var body map[string]any
	if jerr := json.Unmarshal(rec.Body.Bytes(), &body); jerr != nil {
		t.Fatalf("body is not JSON: %v (%q)", jerr, rec.Body.String())
	}
	return rec, body
}

func TestWriteMapsEachValueToItsRow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err    error
		name   string
		status int
	}{
		{Validation{}, "validation", 422},
		{NotFound{}, "not-found", 404},
		{Unauthorized{}, "unauthorized", 401},
		{InvalidCredentials{}, "invalid-credentials", 401},
		{MustChangePassword{}, "must-change-password", 403},
		{ForbiddenOrigin{}, "forbidden-origin", 403},
		{SetupClosed{}, "setup-closed", 409},
		{SetupTokenInvalid{}, "setup-token-invalid", 410},
		{Conflict{Name: "overlap"}, "overlap", 409},
		{Conflict{Name: "duplicate-phone"}, "duplicate-phone", 409},
		{Conflict{Name: "invalid-transition"}, "invalid-transition", 409},
		{PayloadTooLarge{}, "payload-too-large", 413},
		{UnsupportedMediaType{}, "unsupported-media-type", 415},
		{RateLimited{RetryAfter: time.Second}, "rate-limited", 429},
		{errors.New("db down"), "internal", 500},
		{Conflict{Name: "made-up"}, "internal", 500},
		{fmt.Errorf("wrapped: %w", NotFound{}), "not-found", 404},
	}
	for _, tc := range tests {
		t.Run(tc.name+"/"+tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			rec, body := write(t, tc.err)
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Errorf("Content-Type = %q", got)
			}
			if body["type"] != TypePrefix+tc.name {
				t.Errorf("type = %v, want %s", body["type"], TypePrefix+tc.name)
			}
			if body["title"] != rows[tc.name].title {
				t.Errorf("title = %v, want %q", body["title"], rows[tc.name].title)
			}
		})
	}
}

func TestInternalHidesTheError(t *testing.T) {
	t.Parallel()
	rec, _ := write(t, errors.New("pq: password authentication failed"))
	if got := rec.Body.String(); strings.Contains(got, "pq:") || strings.Contains(got, "password") {
		t.Fatalf("internal body leaks the error: %s", got)
	}
}

func TestValidationAlwaysHasAnErrorsArray(t *testing.T) {
	t.Parallel()
	_, body := write(t, Validation{})
	if errs, ok := body["errors"].([]any); !ok || len(errs) != 0 {
		t.Fatalf("errors = %#v, want []", body["errors"])
	}
	_, body = write(t, Validation{Errors: []FieldError{{Field: "phone", Message: "Teléfono no válido"}}})
	want := []any{map[string]any{"field": "phone", "message": "Teléfono no válido"}}
	if fmt.Sprint(body["errors"]) != fmt.Sprint(want) {
		t.Fatalf("errors = %v, want %v", body["errors"], want)
	}
}

func TestRateLimitedSetsRetryAfter(t *testing.T) {
	t.Parallel()
	rec, body := write(t, RateLimited{RetryAfter: 1500 * time.Millisecond})
	if got := rec.Header().Get("Retry-After"); got != "2" {
		t.Errorf("Retry-After = %q, want 2 (rounded up)", got)
	}
	if body["retry_after_s"] != float64(2) {
		t.Errorf("retry_after_s = %v, want 2", body["retry_after_s"])
	}
}

func TestConflictCarriesItsExtensionFields(t *testing.T) {
	t.Parallel()
	_, body := write(t, Conflict{Name: "overlap", Extra: map[string]any{"appointments": []string{"a1"}}})
	if fmt.Sprint(body["appointments"]) != "[a1]" {
		t.Fatalf("appointments = %v, want [a1]", body["appointments"])
	}
}

func TestCollectFields(t *testing.T) {
	t.Parallel()
	errs := []FieldError{{Field: "name", Message: "a"}}
	if !CollectFields(&errs, fmt.Errorf("wrapped: %w", Validation{Errors: []FieldError{{Field: "notes", Message: "b"}}})) {
		t.Fatal("a wrapped Validation must be collected")
	}
	if CollectFields(&errs, errors.New("made-up outage")) || CollectFields(&errs, nil) {
		t.Fatal("a non-Validation error and nil must not be collected")
	}
	if len(errs) != 2 || errs[0].Field != "name" || errs[1].Field != "notes" {
		t.Fatalf("errs = %v, want name then notes", errs)
	}
}
