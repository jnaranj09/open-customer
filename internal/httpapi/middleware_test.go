package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	testVersion = "1.2.3-test"
	testOrigin  = "https://agenda.example.test"
)

// testWrap wraps next with a config like serve's and returns the handler and its log buffer.
func testWrap(t *testing.T, next http.Handler) (http.Handler, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	return wrap(next, middlewareConfig{
		Version:      testVersion,
		PublicOrigin: testOrigin,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
			netip.MustParsePrefix("172.16.0.0/12"),
			netip.MustParsePrefix("192.168.0.0/16"),
		},
		Log: logging.New(&buf, slog.LevelInfo),
	}), &buf
}

// logLines parses the JSON log lines in buf.
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var line map[string]any
		if err := json.Unmarshal(raw, &line); err != nil {
			t.Fatalf("log line is not JSON: %v (%q)", err, raw)
		}
		lines = append(lines, line)
	}
	return lines
}

// lineWith returns the first log line whose key equals value, or nil.
func lineWith(lines []map[string]any, key string, value any) map[string]any {
	for _, l := range lines {
		if l[key] == value {
			return l
		}
	}
	return nil
}

func panics(http.ResponseWriter, *http.Request) { panic("boom: secret panic text") }

func TestXAppVersionOnSuccessErrorAndPanic(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"a 200 response", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }},
		{"a problem response", func(w http.ResponseWriter, r *http.Request) { problem.Write(w, r, problem.NotFound{}) }},
		{"a panic", panics},
	}
	for _, tc := range cases {
		t.Run(tc.name+" carries X-App-Version", func(t *testing.T) {
			t.Parallel()
			h, _ := testWrap(t, tc.handler)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
			if got := rec.Header().Get("X-App-Version"); got != testVersion {
				t.Fatalf("X-App-Version = %q, want %q", got, testVersion)
			}
		})
	}
}

func TestPanicBecomesInternalProblem(t *testing.T) {
	t.Parallel()
	h, buf := testWrap(t, http.HandlerFunc(panics))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["type"] != problem.TypePrefix+"internal" {
		t.Errorf("type = %v, want %s", body["type"], problem.TypePrefix+"internal")
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("body %q leaks the panic text", rec.Body.String())
	}
	logged := lineWith(logLines(t, buf), "code", "panic")
	if logged == nil {
		t.Fatalf("no code=panic log line in %q", buf.String())
	}
	if id, _ := logged["request_id"].(string); len(id) != 32 {
		t.Errorf("panic log request_id = %v, want the 32-character request ID", logged["request_id"])
	}
	if stack, _ := logged["stack"].(string); !strings.Contains(stack, "httpapi.panics") {
		t.Errorf("panic log stack = %v, want a stack through the panicking handler", logged["stack"])
	}
}

func TestRequestLogHasStatusAndRoute(t *testing.T) {
	t.Parallel()
	t.Run("a panic logs status 500", func(t *testing.T) {
		t.Parallel()
		h, buf := testWrap(t, http.HandlerFunc(panics))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
		line := lineWith(logLines(t, buf), "msg", "request")
		if line == nil || line["status"] != float64(http.StatusInternalServerError) {
			t.Fatalf("request log line = %v, want status 500", line)
		}
	})
	t.Run("a routed request logs its mux pattern", func(t *testing.T) {
		t.Parallel()
		mux := http.NewServeMux()
		mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		h, buf := testWrap(t, mux)
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/42", nil))
		line := lineWith(logLines(t, buf), "msg", "request")
		if line == nil || line["route"] != "GET /items/{id}" {
			t.Fatalf("request log line = %v, want route %q", line, "GET /items/{id}")
		}
	})
}

func TestOriginCheck(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		method string
		origin string // "" sends no Origin header
		want   int
	}{
		{"POST without Origin is refused", http.MethodPost, "", http.StatusForbidden},
		{"POST with a wrong Origin is refused", http.MethodPost, "https://evil.example.test", http.StatusForbidden},
		{"POST with a trailing slash on Origin is refused", http.MethodPost, testOrigin + "/", http.StatusForbidden},
		{"POST with the exact Origin passes", http.MethodPost, testOrigin, http.StatusNoContent},
		{"GET without Origin passes", http.MethodGet, "", http.StatusNoContent},
		{"HEAD without Origin passes", http.MethodHead, "", http.StatusNoContent},
		{"PATCH with a wrong Origin is refused", http.MethodPatch, "https://evil.example.test", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, _ := testWrap(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(tc.method, "/x", nil)
			req.Header["Origin"] = []string{tc.origin}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestForbiddenOriginIsProblemAndLogsCodeNotValue(t *testing.T) {
	t.Parallel()
	h, buf := testWrap(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Origin", "https://evil.example.test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["type"] != problem.TypePrefix+"forbidden-origin" {
		t.Errorf("type = %v, want %s", body["type"], problem.TypePrefix+"forbidden-origin")
	}
	if lineWith(logLines(t, buf), "code", "forbidden_origin") == nil {
		t.Errorf("no code=forbidden_origin log line in %q", buf.String())
	}
	if strings.Contains(buf.String(), "evil") {
		t.Errorf("log %q contains the Origin value", buf.String())
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		peer   string
		header string // "" sends no CF-Connecting-IP
		want   string
	}{
		{"a trusted peer's CF-Connecting-IP is used", "172.18.0.5:41000", "203.0.113.7", "203.0.113.7"},
		{"an untrusted peer's CF-Connecting-IP is ignored", "198.51.100.9:41000", "203.0.113.7", "198.51.100.9"},
		{"a trusted peer with an invalid header falls back to the peer", "172.18.0.5:41000", "not-an-ip", "172.18.0.5"},
		{"a trusted peer without the header is the peer", "172.18.0.5:41000", "", "172.18.0.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got netip.Addr
			h, _ := testWrap(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = clientIP(r.Context())
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.RemoteAddr = tc.peer
			req.Header["Cf-Connecting-Ip"] = []string{tc.header}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != netip.MustParseAddr(tc.want) {
				t.Fatalf("clientIP = %v, want %s", got, tc.want)
			}
		})
	}
}

func TestInvalidCFConnectingIPLogsCodeNotValue(t *testing.T) {
	t.Parallel()
	h, buf := testWrap(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "172.18.0.5:41000"
	req.Header.Set("CF-Connecting-IP", "not-an-ip")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if lineWith(logLines(t, buf), "code", "client_ip_invalid") == nil {
		t.Errorf("no code=client_ip_invalid log line in %q", buf.String())
	}
	if strings.Contains(buf.String(), "not-an-ip") {
		t.Errorf("log %q contains the header value", buf.String())
	}
}

func TestMiddlewareOrder(t *testing.T) {
	t.Parallel()
	h, buf := testWrap(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Origin", "https://evil.example.test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 from the Origin check", rec.Code)
	}
	if got := rec.Header().Get("X-App-Version"); got != testVersion {
		t.Errorf("X-App-Version = %q on a refused Origin, want %q", got, testVersion)
	}
	line := lineWith(logLines(t, buf), "msg", "request")
	if line == nil || line["status"] != float64(http.StatusForbidden) {
		t.Errorf("request log line = %v, want one with status 403", line)
	}
}

func TestUnparsablePeerIsNotTrusted(t *testing.T) {
	t.Parallel()
	got := netip.MustParseAddr("192.0.2.1")
	h, buf := testWrap(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = clientIP(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "not-a-peer"
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got.IsValid() {
		t.Errorf("clientIP = %v, want the zero Addr (header not trusted)", got)
	}
	if lineWith(logLines(t, buf), "code", "peer_ip_invalid") == nil {
		t.Errorf("no code=peer_ip_invalid log line in %q", buf.String())
	}
}

func TestRecoverPanicLetsAbortHandlerThrough(t *testing.T) {
	t.Parallel()
	h, _ := testWrap(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	rec := httptest.NewRecorder()

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	}()

	if recovered != http.ErrAbortHandler {
		t.Errorf("panic value = %v, want http.ErrAbortHandler to reach net/http", recovered)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want none written after the abort", rec.Body.String())
	}
}
