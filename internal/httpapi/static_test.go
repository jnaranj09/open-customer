package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const (
	indexBody = "<!doctype html><title>app</title>"
	iconBody  = "\x89PNG-made-up-bytes"
)

func testBuild() fstest.MapFS {
	return fstest.MapFS{
		"index.html":          {Data: []byte(indexBody)},
		"_app/immutable/x.js": {Data: []byte("export {}")},
		"_app/version.json":   {Data: []byte(`{"version":"1"}`)},
		"service-worker.js":   {Data: []byte("// sw")},
		"favicon.png":         {Data: []byte(iconBody)},
	}
}

func getStatic(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	staticHandler(testBuild()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestIndexHTMLIs200NotRedirect(t *testing.T) {
	rec := getStatic(t, "/index.html")
	if rec.Code != http.StatusOK || rec.Body.String() != indexBody {
		t.Fatalf("got %d %q, want 200 with the index body", rec.Code, rec.Body.String())
	}
}

func TestRootServesIndex(t *testing.T) {
	rec := getStatic(t, "/")
	if rec.Code != http.StatusOK || rec.Body.String() != indexBody {
		t.Fatalf("got %d %q, want 200 with the index body", rec.Code, rec.Body.String())
	}
}

func TestUnknownPathFallsBackToIndex(t *testing.T) {
	rec := getStatic(t, "/clientas/123")
	if rec.Code != http.StatusOK || rec.Body.String() != indexBody {
		t.Fatalf("got %d %q, want 200 with the index body", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
}

func TestDirectoryHasNoListing(t *testing.T) {
	rec := getStatic(t, "/_app/")
	if rec.Code != http.StatusOK || rec.Body.String() != indexBody {
		t.Fatalf("got %d %q, want 200 with the index body", rec.Code, rec.Body.String())
	}
}

func TestImmutableCacheHeader(t *testing.T) {
	rec := getStatic(t, "/_app/immutable/x.js")
	want := "public, max-age=31536000, immutable"
	if got := rec.Header().Get("Cache-Control"); got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
}

func TestNoCacheHeaders(t *testing.T) {
	for name, path := range map[string]string{
		"index":          "/index.html",
		"fallback":       "/clientas/123",
		"service worker": "/service-worker.js",
		"version file":   "/_app/version.json",
	} {
		t.Run(name+" is no-cache", func(t *testing.T) {
			rec := getStatic(t, path)
			if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
				t.Fatalf("Cache-Control = %q, want no-cache", got)
			}
		})
	}
}

func TestBuiltFileServed(t *testing.T) {
	rec := getStatic(t, "/favicon.png")
	if rec.Code != http.StatusOK || rec.Body.String() != iconBody {
		t.Fatalf("got %d %q, want 200 with the file bytes", rec.Code, rec.Body.String())
	}
}

func TestMissingIndexIsInternalProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	staticHandler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("Content-Type = %q, want problem+json", ct)
	}
}

// A path with ".." never reaches a file outside the build (threat model T39): the router answers a
// raw ".." with a redirect to the cleaned path, and the cleaned path, like every encoded form, is
// the app shell.
func TestDotDotPathRedirectsToTheCleanPath(t *testing.T) {
	h := newTestApp(t, testServerConfig{}).handler
	rec := serve(h, http.MethodGet, "/../../etc/passwd")
	if rec.Code != http.StatusTemporaryRedirect || rec.Header().Get("Location") != "/etc/passwd" {
		t.Fatalf("got %d Location %q, want 307 to /etc/passwd", rec.Code, rec.Header().Get("Location"))
	}
}

func TestEncodedDotDotPathServesTheAppShell(t *testing.T) {
	h := newTestApp(t, testServerConfig{}).handler
	for _, target := range []string{"/etc/passwd", "/%2e%2e/%2e%2e/etc/passwd", "/_app/%2e%2e/%2e%2e/etc/passwd", "/..%2f..%2fetc/passwd"} {
		t.Run(target, func(t *testing.T) {
			rec := serve(h, http.MethodGet, target)
			if rec.Code != http.StatusOK || rec.Body.String() != indexBody {
				t.Fatalf("got %d %q, want 200 with the index body", rec.Code, rec.Body.String())
			}
		})
	}
}
