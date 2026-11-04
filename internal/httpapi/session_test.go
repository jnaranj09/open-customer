//go:build db

package httpapi

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

const (
	mePath     = "/api/v1/me"
	logoutPath = "/api/v1/auth/logout"
	// changeAccessPath is POST /auth/password; gosec G101 reads a name with "password" in it
	// next to a string as a hard-coded credential.
	changeAccessPath = "/api/v1/auth/password"
	// newOwnerPassword is the made-up password the password-change tests choose.
	newOwnerPassword = "made-up new password"
)

// signedInOwner completes setup on a fresh database and returns the app, the pool and the
// owner's session cookie.
func signedInOwner(t *testing.T) (testApp, *pgxpool.Pool, *http.Cookie) {
	t.Helper()
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	c := sessionCookie(completeSetup(t, app, pool, testOrigin))
	if c == nil {
		t.Fatal("setup set no session cookie")
	}
	return app, pool, c
}

// withCookie adds c to a request (an edit for send).
func withCookie(c *http.Cookie) func(*http.Request) {
	return func(r *http.Request) { r.AddCookie(c) }
}

// getMe sends GET /me with c (none when nil).
func getMe(h http.Handler, c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, mePath, nil)
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// login logs in as "ana" with password and returns the new session cookie.
func login(t *testing.T, app testApp, password string) *http.Cookie {
	t.Helper()
	rec := send(app.handler, http.MethodPost, loginPath, testOrigin, loginBody("ana", password), nil)
	c := sessionCookie(rec)
	if rec.Code != http.StatusOK || c == nil {
		t.Fatalf("POST /auth/login = %d %q, want 200 with a cookie", rec.Code, rec.Body.String())
	}
	return c
}

func passwordBody(current, next string) string {
	return fmt.Sprintf(`{"current_password":%q,"new_password":%q}`, current, next)
}

func assertUnauthorized(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := problemType(t, rec, http.StatusUnauthorized); !strings.HasSuffix(got, ":unauthorized") {
		t.Fatalf("type = %q, want unauthorized", got)
	}
}

func assertNoContent(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("got %d %q, want 204 with no body", rec.Code, rec.Body.String())
	}
}

// mustChange decodes a 200 Me body and returns its must_change_password.
func mustChange(t *testing.T, rec *httptest.ResponseRecorder) bool {
	t.Helper()
	var me gen.Me
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &me) != nil {
		t.Fatalf("GET /me = %d %q, want 200 Me", rec.Code, rec.Body.String())
	}
	return me.MustChangePassword
}

// embeddedSpec is the spec compiled into gen, freshly loaded (a test may change it).
func embeddedSpec(t *testing.T) *openapi3.T {
	t.Helper()
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	return spec
}

func TestMeWithoutCookieIs401(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, getMe(app.handler, nil))
}

func TestMeWithGarbageCookieIs401(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, getMe(app.handler, auth.SessionCookie("not-a-token", true)))
}

func TestMeWithExpiredSessionIs401(t *testing.T) {
	app, _, c := signedInOwner(t)
	app.clock.Advance(91 * 24 * time.Hour)
	assertUnauthorized(t, getMe(app.handler, c))
}

func TestMeOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := getMe(app.handler, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %q, want 200", rec.Code, rec.Body.String())
	}
	assertMe(t, rec, "ana")
}

func TestSlidingCookieResentAfterAnHour(t *testing.T) {
	app, _, c := signedInOwner(t)
	app.clock.Advance(59 * time.Minute)
	if rec := getMe(app.handler, c); rec.Code != http.StatusOK || sessionCookie(rec) != nil {
		t.Fatalf("at 59 min: %d, cookie %+v; want 200 and no Set-Cookie", rec.Code, sessionCookie(rec))
	}
	app.clock.Advance(2 * time.Minute)
	rec := getMe(app.handler, c)
	resent := sessionCookie(rec)
	if rec.Code != http.StatusOK || resent == nil || resent.Value != c.Value || resent.MaxAge != 7776000 {
		t.Fatalf("at 61 min: %d, cookie %+v; want 200 and the same token with Max-Age 7776000", rec.Code, resent)
	}
}

func TestSessionLoadDBErrorIsInternal(t *testing.T) {
	app, pool, c := signedInOwner(t)
	pool.Close()
	if got := problemType(t, getMe(app.handler, c), http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
		t.Fatalf("type = %q, want internal (not a silent sign-out)", got)
	}
}

func TestPublicRoutesNeedNoSession(t *testing.T) {
	app := newTestApp(t, testServerConfig{pool: dbtest.Pool(t)})
	for _, tc := range []struct {
		name, method, path, body string
		want                     int
	}{
		{"GET /setup/status answers without a session", http.MethodGet, "/api/v1/setup/status", "", http.StatusOK},
		{"GET /setup/options answers without a session", http.MethodGet, "/api/v1/setup/options", "", http.StatusOK},
		{"POST /setup reaches its body check without a session", http.MethodPost, setupPath, "{", http.StatusUnprocessableEntity},
		{"POST /auth/login reaches its body check without a session", http.MethodPost, loginPath, "{", http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Origin", testOrigin)
			rec := httptest.NewRecorder()
			app.handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d %q, want %d", rec.Code, rec.Body.String(), tc.want)
			}
		})
	}
}

func TestMustChangePasswordGate(t *testing.T) {
	app, pool, _ := signedInOwner(t)
	temp, err := app.auth.ResetPassword(t.Context(), pool, "ana")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	c := login(t, app, temp)

	if !mustChange(t, getMe(app.handler, c)) {
		t.Fatal("GET /me must_change_password = false after reset-password, want true")
	}
	t.Run("a route outside the allowed set is 403 must-change-password", func(t *testing.T) {
		rec := patchSettings(app.handler, `{"name":"Estudio Nuevo"}`, c)
		if got := problemType(t, rec, http.StatusForbidden); !strings.HasSuffix(got, ":must-change-password") {
			t.Fatalf("type = %q, want must-change-password", got)
		}
	})
	t.Run("a public route is 403 must-change-password too when the cookie is a must-change session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
		req.AddCookie(c)
		rec := httptest.NewRecorder()
		app.handler.ServeHTTP(rec, req)
		if got := problemType(t, rec, http.StatusForbidden); !strings.HasSuffix(got, ":must-change-password") {
			t.Fatalf("type = %q, want must-change-password", got)
		}
	})
	assertNoContent(t, send(app.handler, http.MethodPost, changeAccessPath, testOrigin, passwordBody(temp, newOwnerPassword), withCookie(c)))
	if mustChange(t, getMe(app.handler, c)) {
		t.Fatal("GET /me must_change_password = true after POST /auth/password, want false")
	}
	if rec := patchSettings(app.handler, `{"name":"Estudio Nuevo"}`, c); rec.Code != http.StatusOK {
		t.Fatalf("PATCH /settings after the password change = %d %q, want 200", rec.Code, rec.Body.String())
	}
	assertNoContent(t, send(app.handler, http.MethodPost, logoutPath, testOrigin, "", withCookie(c)))
}

func TestAllowedWhileMustChangeSetMatchesSpec(t *testing.T) {
	allowed, err := allowedWhileMustChangePassword(embeddedSpec(t))
	if err != nil {
		t.Fatalf("allowedWhileMustChangePassword: %v", err)
	}
	got := slices.Sorted(maps.Keys(allowed))
	want := []string{"GET /api/v1/me", "POST /api/v1/auth/logout", "POST /api/v1/auth/password"}
	if !slices.Equal(got, want) {
		t.Fatalf("allowed while must_change_password = %v, want exactly %v", got, want)
	}
}

func TestAllowedWhileMustChangeRejectsANonBoolean(t *testing.T) {
	spec := embeddedSpec(t)
	spec.Paths.Find("/me").Get.Extensions[mustChangeExtension] = "yes"
	if _, err := allowedWhileMustChangePassword(spec); err == nil {
		t.Fatal("a non-boolean extension value was accepted, want an error")
	}
}

func TestLogoutExpiresCookieAndRevokes(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := send(app.handler, http.MethodPost, logoutPath, testOrigin, "", withCookie(c))
	assertNoContent(t, rec)
	if expired := sessionCookie(rec); expired == nil || expired.Value != "" || expired.MaxAge >= 0 {
		t.Fatalf("cookie = %+v, want an expired oc_session", expired)
	}
	assertUnauthorized(t, getMe(app.handler, c))
}

func TestLogoutWithoutOriginIs403(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := send(app.handler, http.MethodPost, logoutPath, "", "", withCookie(c))
	if got := problemType(t, rec, http.StatusForbidden); !strings.HasSuffix(got, ":forbidden-origin") {
		t.Fatalf("type = %q, want forbidden-origin", got)
	}
	if rec := getMe(app.handler, c); rec.Code != http.StatusOK {
		t.Fatalf("GET /me after the refused logout = %d, want 200 (session kept)", rec.Code)
	}
}

func TestChangePasswordWrongCurrentIs422Field(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := send(app.handler, http.MethodPost, changeAccessPath, testOrigin, passwordBody("made-up wrong password", newOwnerPassword), withCookie(c))
	if got := problemType(t, rec, http.StatusUnprocessableEntity); !strings.HasSuffix(got, ":validation") {
		t.Fatalf("type = %q, want validation", got)
	}
	if fields := fieldNames(t, rec); !slices.Equal(fields, []string{"current_password"}) {
		t.Fatalf("errors[].field = %v, want [current_password]", fields)
	}
	if rec := getMe(app.handler, c); rec.Code != http.StatusOK {
		t.Fatalf("GET /me after the wrong current password = %d, want 200 (session kept)", rec.Code)
	}
}

func TestChangePasswordWrongCurrentDelaysTheNextTry(t *testing.T) {
	app, _, c := signedInOwner(t)
	wrong := send(app.handler, http.MethodPost, changeAccessPath, testOrigin, passwordBody("made-up wrong password", newOwnerPassword), withCookie(c))
	if wrong.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong current password = %d %q, want 422", wrong.Code, wrong.Body.String())
	}
	assertRateLimited(t, send(app.handler, http.MethodPost, changeAccessPath, testOrigin, passwordBody(ownerPassword, newOwnerPassword), withCookie(c)))
	app.clock.Advance(time.Second)
	assertNoContent(t, send(app.handler, http.MethodPost, changeAccessPath, testOrigin, passwordBody(ownerPassword, newOwnerPassword), withCookie(c)))
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	app, _, c := signedInOwner(t)
	other := login(t, app, ownerPassword)
	assertNoContent(t, send(app.handler, http.MethodPost, changeAccessPath, testOrigin, passwordBody(ownerPassword, newOwnerPassword), withCookie(c)))
	assertUnauthorized(t, getMe(app.handler, other))
	if rec := getMe(app.handler, c); rec.Code != http.StatusOK {
		t.Fatalf("GET /me with the session that changed the password = %d, want 200", rec.Code)
	}
}

func TestChangePasswordBadInput(t *testing.T) {
	app, _, c := signedInOwner(t)
	for _, tc := range []struct {
		name, body string
		status     int
		suffix     string
	}{
		{"a malformed body is 422", `{"current_password":`, http.StatusUnprocessableEntity, ":validation"},
		{"an unknown field is 422", `{"current_password":"a","new_password":"b","user":"x"}`, http.StatusUnprocessableEntity, ":validation"},
		{"an oversized body is 413", `{"current_password":"` + strings.Repeat("a", maxJSONBody) + `"}`,
			http.StatusRequestEntityTooLarge, ":payload-too-large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := send(app.handler, http.MethodPost, changeAccessPath, testOrigin, tc.body, withCookie(c))
			if got := problemType(t, rec, tc.status); !strings.HasSuffix(got, tc.suffix) {
				t.Fatalf("type = %q, want %s", got, tc.suffix)
			}
		})
	}
}

func TestChangePasswordWithoutSessionIs401(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, send(app.handler, http.MethodPost, changeAccessPath, testOrigin, passwordBody(ownerPassword, newOwnerPassword), nil))
}

// TestLogoutWithoutSessionIs401: the Logout handler has no 401 of its own (it takes the session
// step 6 loaded), so this 401 comes from step 7 alone.
func TestLogoutWithoutSessionIs401(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, send(app.handler, http.MethodPost, logoutPath, testOrigin, "", nil))
}

// pathParam matches a path parameter; the spec-driven test fills each with a made-up UUID.
var pathParam = regexp.MustCompile(`\{[^}]+\}`)

func TestEveryOperationNeedsASessionUnlessPublic(t *testing.T) {
	app := newTestApp(t, testServerConfig{pool: dbtest.Pool(t)})
	spec := embeddedSpec(t)
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			public := op.Security != nil && len(*op.Security) == 0
			t.Run(fmt.Sprintf("%s %s without a cookie is 401 unless security is empty (public=%v)", method, path, public), func(t *testing.T) {
				target := apiBase + pathParam.ReplaceAllString(path, "00000000-0000-4000-8000-000000000001") + requiredQuery(op)
				req := httptest.NewRequest(method, target, strings.NewReader("{"))
				req.Header.Set("Origin", testOrigin)
				rec := httptest.NewRecorder()
				app.handler.ServeHTTP(rec, req)
				if got := rec.Code == http.StatusUnauthorized; got == public {
					t.Fatalf("status = %d %q; want 401 exactly when the operation is not public", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

// requiredQuery is the query string holding a valid value for each required query parameter of
// op. The generated router binds parameters before the session steps run, so a request without
// them would be a 422 and never reach the session check this file tests.
func requiredQuery(op *openapi3.Operation) string {
	var pairs []string
	for _, p := range op.Parameters {
		if p.Value.In != "query" || !p.Value.Required {
			continue
		}
		value := "1"
		if p.Value.Schema.Value.Format == "date" {
			value = "2026-01-01"
		}
		pairs = append(pairs, p.Value.Name+"="+value)
	}
	if len(pairs) == 0 {
		return ""
	}
	return "?" + strings.Join(pairs, "&")
}

func TestPublicOperationsAreExactlyTheSetupAndLoginOnes(t *testing.T) {
	spec := embeddedSpec(t)
	public, err := publicOperations(spec)
	if err != nil {
		t.Fatalf("publicOperations: %v", err)
	}
	var ids []string
	if err := forEachOperation(spec, func(pattern string, op *openapi3.Operation) error {
		if public[pattern] {
			ids = append(ids, op.OperationID)
		}
		return nil
	}); err != nil {
		t.Fatalf("forEachOperation: %v", err)
	}
	slices.Sort(ids)
	// The embedded spec carries the operationIds as oapi-codegen spells them (upper-case first).
	if want := []string{"CompleteSetup", "GetSetupOptions", "GetSetupStatus", "Login"}; !slices.Equal(ids, want) || len(public) != len(want) {
		t.Fatalf("public operations = %v (%d patterns), want exactly %v", ids, len(public), want)
	}
}

func TestPublicOperationsRejectsSecurityStep7CannotCheck(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*openapi3.T)
	}{
		{"an operation naming another scheme is refused", func(spec *openapi3.T) {
			spec.Paths.Find("/me").Get.Security = &openapi3.SecurityRequirements{{"bearer": {}}}
		}},
		{"an operation allowing anonymous access with {} is refused", func(spec *openapi3.T) {
			spec.Paths.Find("/me").Get.Security = &openapi3.SecurityRequirements{{}}
		}},
		{"a global requirement naming another scheme is refused", func(spec *openapi3.T) {
			spec.Security = openapi3.SecurityRequirements{{"bearer": {}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := embeddedSpec(t)
			tc.mutate(spec)
			if _, err := publicOperations(spec); err == nil {
				t.Fatal("publicOperations accepted it, want an error")
			}
		})
	}
}

func TestRequireSessionOnAPatternOutsideThePublicSetIs401(t *testing.T) {
	public, err := publicOperations(embeddedSpec(t))
	if err != nil {
		t.Fatalf("publicOperations: %v", err)
	}
	for _, tc := range []struct{ name, pattern string }{
		{"an empty pattern needs a session", ""},
		{"a pattern the spec does not have needs a session", "DELETE /api/v1/not-in-the-spec"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
			req.Pattern = tc.pattern
			rec := httptest.NewRecorder()
			requireSession(public)(http.NotFoundHandler()).ServeHTTP(rec, req)
			assertUnauthorized(t, rec)
		})
	}
}

// TestSlideCommitFailureIsAnErrorWithNoCookie: the step-6 transaction (the sliding write) whose
// commit fails is a 500, and the cookie is not re-sent.
func TestSlideCommitFailureIsAnErrorWithNoCookie(t *testing.T) {
	_, pool, c := signedInOwner(t)
	app := newTestApp(t, testServerConfig{pool: pool, beginner: failingCommit{pool}})
	app.clock.Advance(61 * time.Minute)
	rec := getMe(app.handler, c)
	if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
		t.Fatalf("type = %q, want internal", got)
	}
	if resent := sessionCookie(rec); resent != nil {
		t.Fatalf("Set-Cookie oc_session = %+v after a failed commit, want none", resent)
	}
}
