package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/exporter"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/media"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/push"
	"github.com/jnaranj09/open-customer/internal/reminders"
	"github.com/jnaranj09/open-customer/internal/scheduler"
	"github.com/jnaranj09/open-customer/internal/services"
	"github.com/jnaranj09/open-customer/internal/settings"
	"github.com/jnaranj09/open-customer/internal/status"
	"github.com/jnaranj09/open-customer/internal/templates"
)

// testStart is the fake clock's time in every test server.
var testStart = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

// testServerConfig is what a test can change in newTestApp; the zero value is newTestServer.
type testServerConfig struct {
	pool      *pgxpool.Pool // nil when no route the test calls touches the database
	publicURL string        // PUBLIC_URL; empty means testOrigin
	trusted   []netip.Prefix
	beginner  db.Beginner // nil means the pool
	mediaDir  string      // MEDIA_DIR; empty means a fresh t.TempDir()
	// afterSettings, when set, runs inside the settings creator after settings.Create, in the
	// setup transaction; it lets a test make a later setup statement fail for real.
	afterSettings func(ctx context.Context, q db.DBTX) error
}

// testApp is the full handler, the auth module behind it and their fake clock.
type testApp struct {
	handler http.Handler
	auth    *auth.Auth
	clock   *clock.Fake
}

// newTestApp builds the full handler like serve does, with real modules on a fake clock.
func newTestApp(t *testing.T, cfg testServerConfig) testApp {
	t.Helper()
	if cfg.publicURL == "" {
		cfg.publicURL = testOrigin
	}
	public, err := url.Parse(cfg.publicURL)
	if err != nil {
		t.Fatalf("parse PUBLIC_URL: %v", err)
	}
	if cfg.mediaDir == "" {
		cfg.mediaDir = t.TempDir()
	}
	fake := clock.NewFake(testStart)
	servicesModule := services.New(services.Options{Clock: fake})
	settingsModule := settings.New(settings.Options{Clock: fake, MediaDir: cfg.mediaDir})
	attrsModule := attributes.New(attributes.Options{Clock: fake})
	a, err := auth.New(auth.Options{
		Clock: fake,
		// The same conversion serve's settingsCreator makes.
		SettingsCreator: func(ctx context.Context, q db.DBTX, s auth.SetupSettings) error {
			if _, err := settingsModule.Create(ctx, q, settings.SettingsInput{
				Name: s.Name, Timezone: s.Timezone, Locale: s.Locale, Currency: s.Currency,
				CountryCode: s.CountryCode, AccentColor: s.AccentColor, PushHour: s.PushHour,
			}); err != nil {
				return fmt.Errorf("create settings: %w", err)
			}
			if cfg.afterSettings != nil {
				return cfg.afterSettings(ctx, q)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	handler, err := New(Options{
		Pool:           cfg.pool,
		Version:        testVersion,
		PublicURL:      public,
		TrustedProxies: cfg.trusted,
		Static:         testBuild(),
		Auth:           a,
		Settings:       settingsModule,
		Attributes:     attrsModule,
		Services:       servicesModule,
		Clients:        clients.New(clients.Options{Clock: fake, Attributes: attrsModule}),
		Appointments:   appointments.New(appointments.Options{Clock: fake, Services: servicesModule, Attributes: attrsModule}),
		Media:          media.New(media.Options{Clock: fake, MediaDir: cfg.mediaDir}),
		Templates:      templates.New(templates.Options{Clock: fake, Attributes: attrsModule}),
		Reminders:      reminders.New(reminders.Options{Clock: fake, Attributes: attrsModule}),
		Push:           push.New(push.Options{Clock: fake, Subject: "mailto:dev@example.com", Resolver: testDNS}),
		Status:         testStatus(t, fake, cfg.mediaDir),
		Exporter:       exporter.New(exporter.Options{Clock: fake, Settings: settingsModule, Attributes: attrsModule}),
		Clock:          fake,
		Log:            logging.New(io.Discard, slog.LevelInfo),
		beginner:       cfg.beginner,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return testApp{handler: handler, auth: a, clock: fake}
}

// fakeDNS answers each host from the map; a host it does not hold is a lookup error.
type fakeDNS map[string][]netip.Addr

func (d fakeDNS) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	found, ok := d[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	return found, nil
}

// testDNS is the DNS the test server's push module sees, so no test makes a real lookup.
// 203.0.113.10 is a documentation address; the push module treats it as public.
var testDNS = fakeDNS{
	"push.example.com":   {netip.MustParseAddr("203.0.113.10")},
	"router.example.com": {netip.MustParseAddr("192.168.1.1")},
}

// testStatus is the status module on clk. Its backup status file does not exist, so the backup line
// is red.
func testStatus(t *testing.T, clk clock.Clock, mediaDir string) *status.Status {
	t.Helper()
	return status.New(status.Options{
		Version: testVersion, Clock: clk, MediaDir: mediaDir,
		BackupStatusFile: filepath.Join(t.TempDir(), "status.json"),
		Push:             push.New(push.Options{Clock: clk, Subject: "mailto:dev@example.com"}),
		Scheduler:        scheduler.New(scheduler.Options{Clock: clk}),
	})
}

// newTestServer is newTestApp's handler with the default config, on pool.
func newTestServer(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	return newTestApp(t, testServerConfig{pool: pool}).handler
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

// The middleware can answer any route with 403 forbidden-origin, 403 must-change-password or
// 500 internal, so every operation declares a default Problem response and the web types never
// promise only the listed statuses.
func TestEveryOperationHasADefaultProblemResponse(t *testing.T) {
	const problemRef = "#/components/responses/Problem"
	var missing []string
	if err := forEachOperation(loadSpec(t), func(pattern string, op *openapi3.Operation) error {
		if def := op.Responses.Default(); def == nil || def.Ref != problemRef {
			missing = append(missing, pattern)
		}
		return nil
	}); err != nil {
		t.Fatalf("forEachOperation: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("operations without default: $ref %s: %v", problemRef, missing)
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
		// Postgres text cannot hold U+0000: without this the insert fails and the route answers 500.
		{"a NUL character in a string is 422 validation", `{"name":"a\u0000b"}`, 422, "validation", "body_nul"},
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

// An escaped backslash followed by "u0000" is plain text, not a NUL character.
func TestDecodeJSONAcceptsTheTextBackslashU0000(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/decode-test", strings.NewReader(`{"name":"a\\u0000b"}`))
	var v decodeTarget
	if err := decodeJSON(httptest.NewRecorder(), req, &v); err != nil {
		t.Fatalf("decodeJSON = %v, want nil", err)
	}
	if v.Name != `a\u0000b` {
		t.Fatalf("name = %q, want the text a\\u0000b", v.Name)
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

func TestDeadlockIsInternalAndLogsOnlyTheSQLState(t *testing.T) {
	logBuf := captureDefaultLog(t)
	rec := httptest.NewRecorder()
	err := fmt.Errorf("db: tx: %w", &pgconn.PgError{
		Code: "40P01", Message: "deadlock on made-up-owner", Detail: "Process 7 waits for made-up-detail",
		TableName: "sessions",
	})
	writeError(rec, httptest.NewRequest(http.MethodPost, "/api/v1/setup", nil), err)
	if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
		t.Fatalf("type = %q, want internal", got)
	}
	logs := logBuf.String()
	if !strings.Contains(logs, "db: tx: ") || !strings.Contains(logs, "40P01") || !strings.Contains(logs, "sessions") {
		t.Fatalf("log = %q, want the wrap path, SQLSTATE 40P01 and the table", logs)
	}
	if strings.Contains(logs, "made-up-owner") || strings.Contains(logs, "made-up-detail") {
		t.Fatalf("log = %q, want neither the Postgres message nor its detail", logs)
	}
}

func TestDBErrorInsideAJoinLogsNoPostgresText(t *testing.T) {
	logBuf := captureDefaultLog(t)
	rec := httptest.NewRecorder()
	err := errors.Join(&pgconn.PgError{Code: "23505", Message: "key made-up-owner exists"}, errors.New("rollback failed"))
	writeError(rec, httptest.NewRequest(http.MethodPost, "/api/v1/setup", nil), err)
	if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
		t.Fatalf("type = %q, want internal", got)
	}
	if logs := logBuf.String(); !strings.Contains(logs, "23505") || strings.Contains(logs, "made-up-owner") {
		t.Fatalf("log = %q, want SQLSTATE 23505 and not the Postgres message", logs)
	}
}

// An earlier part of a join can quote the Postgres message (a caller that formatted it in). The
// text before the PgError is then not a safe wrap path, so none of it is logged.
func TestDBErrorLastInAJoinWhoseEarlierPartQuotesItLogsNoPostgresText(t *testing.T) {
	logBuf := captureDefaultLog(t)
	rec := httptest.NewRecorder()
	pgErr := &pgconn.PgError{Code: "23505", Message: "key made-up-owner exists"}
	err := errors.Join(errors.New("insert failed: key made-up-owner exists"), pgErr)
	writeError(rec, httptest.NewRequest(http.MethodPost, "/api/v1/setup", nil), err)
	if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
		t.Fatalf("type = %q, want internal", got)
	}
	if logs := logBuf.String(); !strings.Contains(logs, "23505") || strings.Contains(logs, "made-up-owner") {
		t.Fatalf("log = %q, want SQLSTATE 23505 and not the Postgres message", logs)
	}
}

// TestNewRefusesAMissingModule: a module serve forgot to pass fails at startup, not on the first
// request that reaches it.
func TestNewRefusesAMissingModule(t *testing.T) {
	public, err := url.Parse(testOrigin)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := auth.New(auth.Options{
		Clock:           clock.NewFake(testStart),
		SettingsCreator: func(context.Context, db.DBTX, auth.SetupSettings) error { return nil },
	})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	attrs := attributes.New(attributes.Options{Clock: clock.NewFake(testStart)})
	servicesModule := services.New(services.Options{Clock: clock.NewFake(testStart)})
	full := Options{
		PublicURL: public, Static: testBuild(), Auth: a,
		Settings:   settings.New(settings.Options{Clock: clock.NewFake(testStart), MediaDir: t.TempDir()}),
		Attributes: attrs,
		Services:   servicesModule,
		Clients:    clients.New(clients.Options{Clock: clock.NewFake(testStart), Attributes: attrs}),
		Appointments: appointments.New(appointments.Options{
			Clock: clock.NewFake(testStart), Services: servicesModule, Attributes: attrs,
		}),
		Media:     media.New(media.Options{Clock: clock.NewFake(testStart), MediaDir: t.TempDir()}),
		Templates: templates.New(templates.Options{Clock: clock.NewFake(testStart), Attributes: attrs}),
		Reminders: reminders.New(reminders.Options{Clock: clock.NewFake(testStart), Attributes: attrs}),
		Push:      push.New(push.Options{Clock: clock.NewFake(testStart), Subject: "mailto:dev@example.com", Resolver: testDNS}),
		Status:    testStatus(t, clock.NewFake(testStart), t.TempDir()),
		Exporter: exporter.New(exporter.Options{
			Clock: clock.NewFake(testStart), Settings: settings.New(settings.Options{Clock: clock.NewFake(testStart)}), Attributes: attrs,
		}),
		Clock: clock.NewFake(testStart),
	}
	for _, tc := range []struct {
		name   string
		remove func(*Options)
	}{
		{"a nil Auth is refused", func(o *Options) { o.Auth = nil }},
		{"a nil Settings is refused", func(o *Options) { o.Settings = nil }},
		{"a nil Attributes is refused", func(o *Options) { o.Attributes = nil }},
		{"a nil Services is refused", func(o *Options) { o.Services = nil }},
		{"a nil Clients is refused", func(o *Options) { o.Clients = nil }},
		{"a nil Appointments is refused", func(o *Options) { o.Appointments = nil }},
		{"a nil Media is refused", func(o *Options) { o.Media = nil }},
		{"a nil Templates is refused", func(o *Options) { o.Templates = nil }},
		{"a nil Reminders is refused", func(o *Options) { o.Reminders = nil }},
		{"a nil Push is refused", func(o *Options) { o.Push = nil }},
		{"a nil Status is refused", func(o *Options) { o.Status = nil }},
		{"a nil Exporter is refused", func(o *Options) { o.Exporter = nil }},
		{"a nil Clock is refused", func(o *Options) { o.Clock = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := full
			tc.remove(&opts)
			if _, err := New(opts); err == nil {
				t.Fatal("New accepted it, want an error")
			}
		})
	}
	if _, err := New(full); err != nil {
		t.Fatalf("New with every module = %v, want nil", err)
	}
}
