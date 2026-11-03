package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/status"
)

// newTestServer builds the full handler like serve does, on pool (nil when no route the test
// calls touches the database).
func newTestServer(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	public, err := url.Parse(testOrigin)
	if err != nil {
		t.Fatalf("parse test origin: %v", err)
	}
	return New(Options{
		Pool:      pool,
		Version:   testVersion,
		PublicURL: public,
		Static:    testBuild(),
		Auth:      auth.New(auth.Options{Clock: clock.NewFake(time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))}),
		Status:    status.New(testVersion),
		Log:       logging.New(io.Discard, slog.LevelInfo),
	})
}

// serve sends one request with the public Origin through h.
func serve(h http.Handler, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Origin", testOrigin)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// problemType asserts rec is problem JSON with status want and returns its type.
func problemType(t *testing.T, rec *httptest.ResponseRecorder, want int) string {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, want, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	return body.Type
}

// captureDefaultLog routes slog's default logger to a buffer until the test ends. Tests that
// use it do not run in parallel.
func captureDefaultLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// loadSpec loads api/openapi.yaml itself: the copy embedded in gen is pruned to the schemas a
// path references, so it lacks Problem.
func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	spec, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("load api/openapi.yaml: %v", err)
	}
	return spec
}

func TestProblemTypesMatchSpec(t *testing.T) {
	spec := loadSpec(t)
	schema, ok := spec.Components.Schemas["Problem"]
	if !ok || schema.Value == nil || schema.Value.Properties["type"] == nil {
		t.Fatal("spec has no Problem.type")
	}
	var fromSpec []string
	for _, v := range schema.Value.Properties["type"].Value.Enum {
		s, _ := v.(string)
		fromSpec = append(fromSpec, strings.TrimPrefix(s, problem.TypePrefix))
	}
	slices.Sort(fromSpec)
	if got := problem.Names(); !slices.Equal(fromSpec, got) {
		t.Fatalf("Problem.type enum in the spec %v differs from problem.Names() %v", fromSpec, got)
	}
}

func TestAPIBaseMatchesSpec(t *testing.T) {
	spec := loadSpec(t)
	if len(spec.Servers) != 1 || spec.Servers[0].URL != apiBase {
		t.Fatalf("spec servers = %v, want exactly %q", spec.Servers, apiBase)
	}
}

type decodeTarget struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestDecodeJSON(t *testing.T) {
	tooLarge := `{"name":"` + strings.Repeat("a", 1<<20) + `"}`
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantType   string
		wantCode   string
	}{
		{"an unknown field is 422 validation", `{"name":"x","extra":1}`, 422, "validation", "body_invalid"},
		{"malformed JSON is 422 validation", `{"name":`, 422, "validation", "body_invalid"},
		{"an empty body is 422 validation", ``, 422, "validation", "body_invalid"},
		{"trailing data after the object is 422 validation", `{"name":"x"}{}`, 422, "validation", "body_invalid"},
		{"a wrong field type is 422 validation", `{"count":"three"}`, 422, "validation", "body_invalid"},
		{"a body over 1 MB is 413 payload-too-large", tooLarge, 413, "payload-too-large", "payload_too_large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureDefaultLog(t)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/decode-test", strings.NewReader(tc.body))
			var v decodeTarget
			if err := decodeJSON(rec, req, &v); err == nil {
				t.Fatal("decodeJSON returned nil, want an error")
			}
			if got := problemType(t, rec, tc.wantStatus); got != problem.TypePrefix+tc.wantType {
				t.Fatalf("type = %q, want %q", got, problem.TypePrefix+tc.wantType)
			}
			if !strings.Contains(logs.String(), `"code":"`+tc.wantCode+`"`) ||
				!strings.Contains(logs.String(), `"path":"/decode-test"`) {
				t.Fatalf("log lacks code=%s and the path: %s", tc.wantCode, logs)
			}
		})
	}
}

// The unknown-field decode error quotes the field name, so logging the error would leak it.
func TestDecodeJSONLogNeverQuotesTheDecodeError(t *testing.T) {
	logs := captureDefaultLog(t)
	req := httptest.NewRequest(http.MethodPost, "/decode-test", strings.NewReader(`{"zzleakfield":1}`))
	var v decodeTarget
	if err := decodeJSON(httptest.NewRecorder(), req, &v); err == nil {
		t.Fatal("decodeJSON returned nil, want an error")
	}
	if !strings.Contains(logs.String(), `"code":"body_invalid"`) || strings.Contains(logs.String(), "zzleakfield") {
		t.Fatalf("log lacks code=body_invalid or quotes the decode error: %s", logs)
	}
}

func TestDecodeJSONValidBody(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/decode-test", strings.NewReader(`{"name":"x","count":3}`))
	var v decodeTarget
	if err := decodeJSON(rec, req, &v); err != nil {
		t.Fatalf("decodeJSON = %v, want nil", err)
	}
	if v != (decodeTarget{Name: "x", Count: 3}) {
		t.Fatalf("decoded %+v, want {x 3}", v)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("decodeJSON wrote %q on success, want nothing", rec.Body.String())
	}
}

func TestErrorHandlerFuncWritesValidation(t *testing.T) {
	logs := captureDefaultLog(t)
	h, _ := testWrap(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bindErrorHandler(w, r, &gen.InvalidParamFormatError{ParamName: "id", Err: errors.New("bad uuid: zz-made-up")})
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bind-test", nil))

	if got := problemType(t, rec, http.StatusUnprocessableEntity); got != problem.TypePrefix+"validation" {
		t.Fatalf("type = %q, want validation", got)
	}
	if got := rec.Header().Get("X-App-Version"); got != testVersion {
		t.Fatalf("X-App-Version = %q, want %q", got, testVersion)
	}
	if !strings.Contains(logs.String(), `"code":"param_invalid"`) || strings.Contains(logs.String(), "zz-made-up") {
		t.Fatalf("log lacks code=param_invalid or leaks the bind error: %s", logs)
	}
	if strings.Contains(rec.Body.String(), "zz-made-up") {
		t.Fatalf("body leaks the bind error: %s", rec.Body.String())
	}
}

func TestUnknownAPIPathIs404Problem(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		t.Run(method+" on an unknown /api/v1 path is a 404 problem", func(t *testing.T) {
			t.Parallel()
			rec := serve(newTestServer(t, nil), method, "/api/v1/nope")
			if got := problemType(t, rec, http.StatusNotFound); got != problem.TypePrefix+"not-found" {
				t.Fatalf("type = %q, want not-found", got)
			}
			if rec.Header().Get("X-App-Version") != testVersion {
				t.Fatalf("X-App-Version = %q, want %q", rec.Header().Get("X-App-Version"), testVersion)
			}
		})
	}
}

func TestWrongMethodIs404Problem(t *testing.T) {
	t.Parallel()
	rec := serve(newTestServer(t, nil), http.MethodDelete, "/api/v1/setup/status")
	if got := problemType(t, rec, http.StatusNotFound); got != problem.TypePrefix+"not-found" {
		t.Fatalf("type = %q, want not-found", got)
	}
}

func TestStaticServesGetAndHeadOnly(t *testing.T) {
	t.Parallel()
	h := newTestServer(t, nil)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method+" on an app path serves the index", func(t *testing.T) {
			t.Parallel()
			rec := serve(h, method, "/clientas/123")
			if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("got %d %q, want 200 text/html", rec.Code, rec.Header().Get("Content-Type"))
			}
		})
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method+" on an app path is 405", func(t *testing.T) {
			t.Parallel()
			rec := serve(h, method, "/clientas/123")
			if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
				t.Fatalf("got %d Allow %q, want 405 Allow \"GET, HEAD\"", rec.Code, rec.Header().Get("Allow"))
			}
			if strings.Contains(rec.Body.String(), indexBody) {
				t.Fatal("405 body is the index")
			}
		})
	}
}

func TestPublicOrigin(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"https://agenda.example.test", "https://agenda.example.test"},
		{"https://agenda.example.test/", "https://agenda.example.test"},
		{"https://x:443/", "https://x"},
		{"https://x:8443", "https://x:8443"},
		{"http://localhost:80", "http://localhost"},
		{"http://localhost:5173", "http://localhost:5173"},
		{"https://Agenda.Example.Test", "https://agenda.example.test"},
	}
	for _, tc := range cases {
		t.Run(tc.in+" is sent by browsers as "+tc.want, func(t *testing.T) {
			t.Parallel()
			u, err := url.Parse(tc.in)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := publicOrigin(u); got != tc.want {
				t.Fatalf("publicOrigin(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
