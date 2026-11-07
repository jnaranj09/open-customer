//go:build db

package httpapi

import (
	"net/http"
	"testing"
)

// TestSecurityHeadersOnEveryResponse: the fixed security headers (devsecops.md section 6, decision
// D8) are on every kind of response the app writes: an API JSON body, a problem, the app shell,
// a built file, a probe, a stored photo and both icons (an upload served as itself, threat model
// T31). Strict-Transport-Security is sent only when PUBLIC_URL is https. No response allows
// another origin to read it (no CORS header). It is db-tagged because the photo and the icons
// come from the database and the media folder.
func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	f := newMediaFixture(t, nil)
	photo := f.uploadOK(t)
	assertNoContent(t, uploadParts(t, f.app.handler, f.cookie, validIcons(t)...))
	signedIn := withCookie(f.cookie)
	responses := []struct {
		name       string
		target     string
		edit       func(*http.Request) // the request edit: nil, or the session cookie
		wantStatus int
	}{
		{"an API JSON response", "/api/v1/setup/options", nil, http.StatusOK},
		{"a problem response", "/api/v1/me", nil, http.StatusUnauthorized},
		{"an unknown API path", "/api/v1/no-such-route", signedIn, http.StatusNotFound},
		{"the app shell", "/", nil, http.StatusOK},
		{"a built file", "/_app/immutable/x.js", nil, http.StatusOK},
		{"the liveness probe", "/healthz", nil, http.StatusOK},
		{"a stored photo", mediaPath + "/" + photo.Id.String(), signedIn, http.StatusOK},
		{"a stored thumbnail", mediaPath + "/" + photo.Id.String() + "/thumb", signedIn, http.StatusOK},
		{"the uploaded 512 px icon", icon512Path, nil, http.StatusOK},
		{"the uploaded apple touch icon", appleTouchIconPath, nil, http.StatusOK},
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
		app := newTestApp(t, testServerConfig{pool: f.pool, publicURL: u.publicURL, mediaDir: f.dir})
		for _, tc := range responses {
			t.Run(tc.name+" "+u.name, func(t *testing.T) {
				// A GET needs no Origin.
				rec := send(app.handler, http.MethodGet, tc.target, "", "", tc.edit)
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
