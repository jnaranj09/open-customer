package httpapi

import (
	"net/http"
	"testing"
)

// TestSecurityHeadersOnEveryResponse: the fixed security headers (devsecops.md section 6, decision
// D8) are on every kind of response the app writes: an API JSON body, a problem, the app shell,
// a built file and a probe. Strict-Transport-Security is sent only when PUBLIC_URL is https. No
// response allows another origin to read it (no CORS header).
func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	t.Parallel()
	responses := []struct {
		name       string
		target     string
		wantStatus int
	}{
		{"an API JSON response", "/api/v1/setup/options", http.StatusOK},
		{"a problem response", "/api/v1/me", http.StatusUnauthorized},
		{"the app shell", "/", http.StatusOK},
		{"a built file", "/_app/immutable/x.js", http.StatusOK},
		{"the liveness probe", "/healthz", http.StatusOK},
	}
	urls := []struct {
		name      string
		publicURL string
		wantHSTS  string
	}{
		{"with an https PUBLIC_URL", testOrigin, "max-age=31536000"},
		{"with an http PUBLIC_URL", "http://localhost:8080", ""},
	}
	for _, u := range urls {
		app := newTestApp(t, testServerConfig{publicURL: u.publicURL})
		for _, tc := range responses {
			t.Run(tc.name+" "+u.name, func(t *testing.T) {
				t.Parallel()
				rec := serve(app.handler, http.MethodGet, tc.target)
				if rec.Code != tc.wantStatus {
					t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
				}
				want := map[string]string{
					"X-Content-Type-Options":      "nosniff",
					"Referrer-Policy":             "same-origin",
					"Content-Security-Policy":     "frame-ancestors 'none'; base-uri 'self'; object-src 'none'; form-action 'self'",
					"Strict-Transport-Security":   u.wantHSTS,
					"Access-Control-Allow-Origin": "",
				}
				for name, value := range want {
					if got := rec.Header().Get(name); got != value {
						t.Errorf("%s = %q, want %q", name, got, value)
					}
				}
			})
		}
	}
}
