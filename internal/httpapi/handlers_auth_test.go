//go:build db

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/settings"
)

const (
	setupPath = "/api/v1/setup"
	loginPath = "/api/v1/auth/login"
	// ownerPassword is the made-up password every setup in these tests chooses.
	ownerPassword = "correct horse battery"
)

// send sends body to target with method and origin as the Origin header (none when empty), after
// edit has changed the request (nil for none; the cookie goes in through it).
func send(h http.Handler, method, target, origin, body string, edit func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if edit != nil {
		edit(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// newSetupToken stores a setup token the way serve does (inside a transaction) and returns it.
func newSetupToken(t *testing.T, app testApp, pool *pgxpool.Pool) string {
	t.Helper()
	var token string
	err := db.WithTx(t.Context(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		token, err = app.auth.NewSetupToken(t.Context(), tx)
		return err
	})
	if err != nil {
		t.Fatalf("NewSetupToken: %v", err)
	}
	return token
}

// setupBody is a valid POST /setup body for token with the made-up owner "ana", with timezone
// as the business time zone.
func setupBody(token, timezone string) string {
	return fmt.Sprintf(`{"token":%q,"username":"Ana","password":%q,"settings":{"name":"Estudio Prueba",`+
		`"timezone":%q,"locale":"es","currency":"eur","country_code":"ES","accent_color":"#AA3366"}}`,
		token, ownerPassword, timezone)
}

// completeSetup runs a successful POST /setup on app and returns the response.
func completeSetup(t *testing.T, app testApp, pool *pgxpool.Pool, origin string) *httptest.ResponseRecorder {
	t.Helper()
	rec := send(app.handler, http.MethodPost, setupPath, origin, setupBody(newSetupToken(t, app, pool), "Europe/Madrid"), nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /setup = %d %q, want 201", rec.Code, rec.Body.String())
	}
	return rec
}

// sessionCookie returns the oc_session cookie rec sets, or nil.
func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oc_session" {
			return c
		}
	}
	return nil
}

// assertMe asserts rec's body is a Me with exactly the fields id, username and
// must_change_password, for username and a must_change_password of false.
func assertMe(t *testing.T, rec *httptest.ResponseRecorder, username string) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if want := []string{"id", "must_change_password", "username"}; !slices.Equal(keys, want) {
		t.Fatalf("body fields = %v, want %v", keys, want)
	}
	if id, ok := body["id"].(string); !ok || len(id) != 36 {
		t.Fatalf("id = %v, want a UUID string", body["id"])
	}
	if body["username"] != username || body["must_change_password"] != false {
		t.Fatalf("body = %v, want username %q and must_change_password false", body, username)
	}
}

// fieldNames returns the errors[].field values of a validation problem.
func fieldNames(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var body struct {
		Errors []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	names := make([]string, len(body.Errors))
	for i, e := range body.Errors {
		names[i] = e.Field
	}
	return names
}

func assertSetupStillOpen(t *testing.T, app testApp, pool *pgxpool.Pool) {
	t.Helper()
	needed, err := app.auth.SetupNeeded(t.Context(), pool)
	if err != nil || !needed {
		t.Fatalf("SetupNeeded = %v, %v; want true, nil (no user created)", needed, err)
	}
}

func TestSetupOptionsListsCountries(t *testing.T) {
	rec := serve(newTestServer(t, nil), http.MethodGet, "/api/v1/setup/options")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %q, want 200", rec.Code, rec.Body.String())
	}
	var body struct {
		Countries []string `json:"countries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if !slices.Equal(body.Countries, settings.Countries()) || !slices.Contains(body.Countries, "ES") {
		t.Fatalf("countries = %v, want settings.Countries() (with ES)", body.Countries)
	}
}

func TestPostSetupCreatesOwner(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool, publicURL: "http://localhost:5173"})
	rec := completeSetup(t, app, pool, "http://localhost:5173")

	assertMe(t, rec, "ana")
	c := sessionCookie(rec)
	if c == nil || c.Value == "" {
		t.Fatalf("no oc_session cookie in %v", rec.Header().Values("Set-Cookie"))
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Secure {
		t.Fatalf("cookie = %+v, want HttpOnly, SameSite=Lax, Path=/ and no Secure on http://localhost", c)
	}
	got, err := settings.New(settings.Options{Clock: clock.NewFake(testStart)}).Get(t.Context(), pool)
	want := settings.Settings{
		Name: "Estudio Prueba", Timezone: "Europe/Madrid", Locale: "es", Currency: "EUR",
		CountryCode: "ES", AccentColor: "#aa3366", PushHour: 8,
	}
	if err != nil || got != want {
		t.Fatalf("settings row = %+v, %v; want %+v", got, err, want)
	}
}

func TestPostSetupCookieIsSecureOnHTTPS(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	if c := sessionCookie(completeSetup(t, app, pool, testOrigin)); c == nil || !c.Secure {
		t.Fatalf("cookie = %+v, want Secure for an https PUBLIC_URL", c)
	}
}

func TestPostSetupTwiceIsSetupClosed(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	token := newSetupToken(t, app, pool)
	if rec := send(app.handler, http.MethodPost, setupPath, testOrigin, setupBody(token, "Europe/Madrid"), nil); rec.Code != http.StatusCreated {
		t.Fatalf("first POST /setup = %d %q, want 201", rec.Code, rec.Body.String())
	}
	rec := send(app.handler, http.MethodPost, setupPath, testOrigin, setupBody(token, "Europe/Madrid"), nil)
	if got := problemType(t, rec, http.StatusConflict); !strings.HasSuffix(got, ":setup-closed") {
		t.Fatalf("type = %q, want setup-closed", got)
	}
	if sessionCookie(rec) != nil {
		t.Fatal("a refused setup set a session cookie")
	}
}

func TestPostSetupBadTokenIs410(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	newSetupToken(t, app, pool)
	rec := send(app.handler, http.MethodPost, setupPath, testOrigin, setupBody("not-the-token", "Europe/Madrid"), nil)
	if got := problemType(t, rec, http.StatusGone); !strings.HasSuffix(got, ":setup-token-invalid") {
		t.Fatalf("type = %q, want setup-token-invalid", got)
	}
	assertSetupStillOpen(t, app, pool)
}

func TestPostSetupUnknownFieldIs422(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	body := strings.Replace(setupBody(newSetupToken(t, app, pool), "Europe/Madrid"), "{", `{"role":"admin",`, 1)
	rec := send(app.handler, http.MethodPost, setupPath, testOrigin, body, nil)
	if got := problemType(t, rec, http.StatusUnprocessableEntity); !strings.HasSuffix(got, ":validation") {
		t.Fatalf("type = %q, want validation", got)
	}
	assertSetupStillOpen(t, app, pool)
}

func TestPostSetupMalformedIs422(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	rec := send(app.handler, http.MethodPost, setupPath, testOrigin, `{"token":`, nil)
	if got := problemType(t, rec, http.StatusUnprocessableEntity); !strings.HasSuffix(got, ":validation") {
		t.Fatalf("type = %q, want validation", got)
	}
	assertSetupStillOpen(t, app, pool)
}

func TestPostSetupOversizeIs413(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	body := `{"token":"` + strings.Repeat("a", maxJSONBody) + `"}`
	rec := send(app.handler, http.MethodPost, setupPath, testOrigin, body, nil)
	if got := problemType(t, rec, http.StatusRequestEntityTooLarge); !strings.HasSuffix(got, ":payload-too-large") {
		t.Fatalf("type = %q, want payload-too-large", got)
	}
}

func TestPostSetupBadSettingsIs422WithFields(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	rec := send(app.handler, http.MethodPost, setupPath, testOrigin, setupBody(newSetupToken(t, app, pool), "Mars/Olympus"), nil)
	if got := problemType(t, rec, http.StatusUnprocessableEntity); !strings.HasSuffix(got, ":validation") {
		t.Fatalf("type = %q, want validation", got)
	}
	if fields := fieldNames(t, rec); !slices.Contains(fields, "settings.timezone") {
		t.Fatalf("errors[].field = %v, want settings.timezone among them", fields)
	}
	assertSetupStillOpen(t, app, pool)
}

func TestPostSetupNoOriginIs403(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	rec := send(app.handler, http.MethodPost, setupPath, "", setupBody(newSetupToken(t, app, pool), "Europe/Madrid"), nil)
	if got := problemType(t, rec, http.StatusForbidden); !strings.HasSuffix(got, ":forbidden-origin") {
		t.Fatalf("type = %q, want forbidden-origin", got)
	}
	assertSetupStillOpen(t, app, pool)
}

// TestPostSetupDBErrorLogsNoRawMessage: a unique violation (here the settings creator hook
// inserts the same username first, so the owner user insert fails) is a 500 internal. The log keeps the wrap path, the SQLSTATE, the constraint and
// the table, and names neither the username nor the Postgres message.
func TestPostSetupDBErrorLogsNoRawMessage(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool, afterSettings: func(ctx context.Context, q db.DBTX) error {
		return sqlc.New(q).InsertUser(ctx, sqlc.InsertUserParams{
			ID: [16]byte{1}, Username: "ana", PasswordHash: "x", CreatedAt: testStart,
		})
	}})
	logBuf := captureDefaultLog(t)
	rec := send(app.handler, http.MethodPost, setupPath, testOrigin, setupBody(newSetupToken(t, app, pool), "Europe/Madrid"), nil)
	if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
		t.Fatalf("type = %q, want internal", got)
	}
	logs := logBuf.String()
	for _, want := range []string{"insert the owner user", "23505", "users_username_key", "users"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("log = %q, want it to keep %q (the wrap path, SQLSTATE, constraint and table)", logs, want)
		}
	}
	if strings.Contains(logs, "ana") || strings.Contains(logs, "duplicate") {
		t.Fatalf("log = %q, want neither the username nor the Postgres message", logs)
	}
	assertSetupStillOpen(t, app, pool)
}

func loginBody(username, password string) string {
	return fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
}

func TestLoginOK(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	completeSetup(t, app, pool, testOrigin)
	rec := send(app.handler, http.MethodPost, loginPath, testOrigin, loginBody(" ANA ", ownerPassword), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %q, want 200", rec.Code, rec.Body.String())
	}
	assertMe(t, rec, "ana")
	if c := sessionCookie(rec); c == nil || c.Value == "" || !c.HttpOnly || !c.Secure {
		t.Fatalf("cookie = %+v, want an HttpOnly, Secure oc_session", c)
	}
}

func TestLoginWrongPasswordIs401InvalidCredentials(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	completeSetup(t, app, pool, testOrigin)
	rec := send(app.handler, http.MethodPost, loginPath, testOrigin, loginBody("ana", "wrong password"), nil)
	if got := problemType(t, rec, http.StatusUnauthorized); !strings.HasSuffix(got, ":invalid-credentials") {
		t.Fatalf("type = %q, want invalid-credentials", got)
	}
	if sessionCookie(rec) != nil {
		t.Fatal("a failed login set a session cookie")
	}
}

func TestLoginBadInputIs422(t *testing.T) {
	app := newTestApp(t, testServerConfig{pool: dbtest.Pool(t)})
	for _, tc := range []struct{ name, body string }{
		{"a malformed body is refused", `{"username":`},
		{"an unknown field is refused", `{"username":"ana","password":"x","remember":true}`},
		{"a username with a space inside is refused", loginBody("a na", ownerPassword)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := send(app.handler, http.MethodPost, loginPath, testOrigin, tc.body, nil)
			if got := problemType(t, rec, http.StatusUnprocessableEntity); !strings.HasSuffix(got, ":validation") {
				t.Fatalf("type = %q, want validation", got)
			}
		})
	}
}

// failedLogins sends n logins, each for a different unknown username (so only the IP bucket
// counts, not a username's delay), with edit applied to each request, and asserts each is 401.
func failedLogins(t *testing.T, h http.Handler, n int, edit func(i int) func(*http.Request)) {
	t.Helper()
	for i := range n {
		rec := send(h, http.MethodPost, loginPath, testOrigin, loginBody("nobody"+strconv.Itoa(i), "wrong password"), edit(i))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d %q, want 401", i+1, rec.Code, rec.Body.String())
		}
	}
}

// from sets the TCP peer and, when cf is not empty, CF-Connecting-IP.
func from(peer, cf string) func(*http.Request) {
	return func(r *http.Request) {
		r.RemoteAddr = peer
		if cf != "" {
			r.Header.Set("CF-Connecting-IP", cf)
		}
	}
}

func assertRateLimited(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := problemType(t, rec, http.StatusTooManyRequests); !strings.HasSuffix(got, ":rate-limited") {
		t.Fatalf("type = %q, want rate-limited", got)
	}
	var body struct {
		RetryAfterS int `json:"retry_after_s"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.RetryAfterS < 1 {
		t.Fatalf("retry_after_s = %d (%v), want at least 1", body.RetryAfterS, err)
	}
	if got := rec.Header().Get("Retry-After"); got != strconv.Itoa(body.RetryAfterS) {
		t.Fatalf("Retry-After = %q, want %d", got, body.RetryAfterS)
	}
}

func TestLoginOversizeIs413(t *testing.T) {
	app := newTestApp(t, testServerConfig{pool: dbtest.Pool(t)})
	body := `{"username":"` + strings.Repeat("a", maxJSONBody) + `"}`
	rec := send(app.handler, http.MethodPost, loginPath, testOrigin, body, nil)
	if got := problemType(t, rec, http.StatusRequestEntityTooLarge); !strings.HasSuffix(got, ":payload-too-large") {
		t.Fatalf("type = %q, want payload-too-large", got)
	}
}

func TestLoginSixthAttemptIs429(t *testing.T) {
	app := newTestApp(t, testServerConfig{pool: dbtest.Pool(t)})
	same := func(int) func(*http.Request) { return from("192.0.2.1:4000", "") }
	failedLogins(t, app.handler, 5, same)
	assertRateLimited(t, send(app.handler, http.MethodPost, loginPath, testOrigin, loginBody("nobody-else", "x"), same(0)))
}

// The limits are checked before the transaction: a refused login opens none, so a burst of
// refused logins holds no database connection.
func TestLoginRateLimitedOpensNoTransaction(t *testing.T) {
	pool := dbtest.Pool(t)
	rec := &dbtest.TxModeRecorder{Pool: pool}
	app := newTestApp(t, testServerConfig{pool: pool, beginner: rec})
	same := func(int) func(*http.Request) { return from("192.0.2.1:4000", "") }
	failedLogins(t, app.handler, 5, same)
	before := len(rec.Modes())
	assertRateLimited(t, send(app.handler, http.MethodPost, loginPath, testOrigin, loginBody("nobody-else", "x"), same(0)))
	if got := len(rec.Modes()) - before; got != 0 {
		t.Fatalf("a rate-limited login opened %d transactions; want 0", got)
	}
}

func TestLoginRateLimitTrustedVsUntrustedPeer(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}

	t.Run("behind a trusted proxy each CF-Connecting-IP has its own bucket", func(t *testing.T) {
		app := newTestApp(t, testServerConfig{pool: dbtest.Pool(t), trusted: trusted})
		failedLogins(t, app.handler, 5, func(int) func(*http.Request) { return from("172.18.0.5:4000", "203.0.113.7") })
		assertRateLimited(t, send(app.handler, http.MethodPost, loginPath, testOrigin, loginBody("nobody-a", "x"),
			from("172.18.0.5:4000", "203.0.113.7")))
		rec := send(app.handler, http.MethodPost, loginPath, testOrigin, loginBody("nobody-b", "x"), from("172.18.0.5:4000", "203.0.113.8"))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("another client behind the proxy = %d %q, want 401 (not rate limited)", rec.Code, rec.Body.String())
		}
	})

	t.Run("from an untrusted peer a varying CF-Connecting-IP shares the peer's bucket", func(t *testing.T) {
		app := newTestApp(t, testServerConfig{pool: dbtest.Pool(t), trusted: trusted})
		failedLogins(t, app.handler, 5, func(i int) func(*http.Request) {
			return from("198.51.100.9:4000", "203.0.113."+strconv.Itoa(i+1))
		})
		assertRateLimited(t, send(app.handler, http.MethodPost, loginPath, testOrigin, loginBody("nobody-c", "x"),
			from("198.51.100.9:4000", "203.0.113.99")))
	})
}

// failingCommit begins real transactions whose Commit rolls back and fails, as when the
// connection drops at commit.
type failingCommit struct{ pool *pgxpool.Pool }

var errCommitFailed = errors.New("commit failed (test)")

func (f failingCommit) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, err := f.pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	return failingCommitTx{tx}, nil
}

type failingCommitTx struct{ pgx.Tx }

func (tx failingCommitTx) Commit(ctx context.Context) error {
	if err := tx.Rollback(ctx); err != nil {
		return fmt.Errorf("rollback in place of commit: %w", err)
	}
	return errCommitFailed
}

func TestCommitFailureIsNotShownAsSaved(t *testing.T) {
	t.Run("a setup whose commit fails is an error with no cookie", func(t *testing.T) {
		pool := dbtest.Pool(t)
		app := newTestApp(t, testServerConfig{pool: pool, beginner: failingCommit{pool}})
		rec := send(app.handler, http.MethodPost, setupPath, testOrigin, setupBody(newSetupToken(t, app, pool), "Europe/Madrid"), nil)
		if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
			t.Fatalf("type = %q, want internal", got)
		}
		if sessionCookie(rec) != nil {
			t.Fatal("a setup whose commit failed set a session cookie")
		}
		assertSetupStillOpen(t, app, pool)
	})

	t.Run("a login whose commit fails is an error with no cookie", func(t *testing.T) {
		pool := dbtest.Pool(t)
		completeSetup(t, newTestApp(t, testServerConfig{pool: pool}), pool, testOrigin)
		app := newTestApp(t, testServerConfig{pool: pool, beginner: failingCommit{pool}})
		rec := send(app.handler, http.MethodPost, loginPath, testOrigin, loginBody("ana", ownerPassword), nil)
		if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
			t.Fatalf("type = %q, want internal", got)
		}
		if sessionCookie(rec) != nil {
			t.Fatal("a login whose commit failed set a session cookie")
		}
	})
}
