package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/settings"
	"github.com/jnaranj09/open-customer/internal/status"
)

// apiBase is servers[0].url of api/openapi.yaml; TestAPIBaseMatchesSpec keeps the two equal.
const apiBase = "/api/v1"

// maxJSONBody is the limit decodeJSON puts on every JSON request body. Multipart handlers set
// their own limits.
const maxJSONBody = 1 << 20

// Options is what serve passes to New.
type Options struct {
	Pool    *pgxpool.Pool
	Version string // sent as X-App-Version on every response
	// PublicURL is PUBLIC_URL; its origin is the only Origin accepted on writes.
	PublicURL      *url.URL
	TrustedProxies []netip.Prefix // TRUSTED_PROXY_CIDRS
	Static         fs.FS          // the built SPA (fs.Sub of web.Build)
	Auth           *auth.Auth
	Settings       *settings.Module
	Status         *status.Status
	Log            *slog.Logger
	// beginner opens the write transactions; nil means Pool. Only tests set it (a Commit that
	// fails).
	beginner db.Beginner
}

// Server implements gen.ServerInterface over the module values (design doc item 32).
type Server struct {
	pool     *pgxpool.Pool
	beginner db.Beginner // begins every write transaction (db.WithTx)
	auth     *auth.Auth
	settings *settings.Module
	status   *status.Status
	// secureCookies is PUBLIC_URL's scheme == https: the session cookie drops Secure only for a
	// plain-http PUBLIC_URL (localhost in development and e2e).
	secureCookies bool
}

var _ gen.ServerInterface = (*Server)(nil)

// New returns the app's whole HTTP handler: the probes, the manifest and icons, the generated
// /api/v1 routes (behind the session steps 6–8), the /api/ catch-all and the SPA, inside the
// middleware steps 1–5 of design doc item 34. It fails when a module (Auth, Settings, Status) is
// missing, or when the embedded spec cannot be read or its security or
// x-oc-allowed-while-must-change-password values are not ones steps 7–8 can enforce.
func New(opts Options) (http.Handler, error) {
	if opts.Auth == nil || opts.Settings == nil || opts.Status == nil {
		return nil, errors.New("httpapi: Options.Auth, Settings and Status are required")
	}
	spec, err := gen.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("httpapi: load the embedded spec: %w", err)
	}
	public, err := publicOperations(spec)
	if err != nil {
		return nil, err
	}
	allowed, err := allowedWhileMustChangePassword(spec)
	if err != nil {
		return nil, err
	}
	s := &Server{
		pool: opts.Pool, beginner: opts.beginner,
		auth: opts.Auth, settings: opts.Settings, status: opts.Status,
		secureCookies: opts.PublicURL.Scheme == "https",
	}
	if s.beginner == nil {
		s.beginner = opts.Pool
	}
	mux := http.NewServeMux()
	// The probes are outside /api/v1 and the spec, so they are registered by literal path.
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	// The manifest and the icons are outside /api/v1 and the spec (the browser asks for them at
	// fixed paths), so they are registered by literal path, before /, and with no session: the
	// browser and iOS fetch them without cookies, also before setup.
	mux.HandleFunc("GET /manifest.webmanifest", s.manifest)
	mux.HandleFunc("GET /icons/512.png", s.icon512)
	mux.HandleFunc("GET /apple-touch-icon.png", s.appleTouchIcon)
	gen.HandlerWithOptions(s, gen.StdHTTPServerOptions{
		BaseURL:          apiBase,
		BaseRouter:       mux,
		ErrorHandlerFunc: bindErrorHandler,
		Middlewares:      s.sessionMiddlewares(public, allowed),
	})
	// Every other path and method under /api/ is a 404 problem. Without this, the / pattern
	// below would answer it with index.html and a 200.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		problem.Write(w, r, problem.NotFound{})
	})
	// "GET /" would conflict with "/api/" in the mux, so / matches every method and
	// getOrHeadOnly refuses the others.
	mux.Handle("/", getOrHeadOnly(staticHandler(opts.Static)))
	return wrap(mux, middlewareConfig{
		Version:        opts.Version,
		PublicOrigin:   publicOrigin(opts.PublicURL),
		TrustedProxies: opts.TrustedProxies,
		Log:            opts.Log,
	}), nil
}

// getOrHeadOnly answers every method except GET and HEAD with 405.
func getOrHeadOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// publicOrigin serializes u the way a browser sends Origin: scheme://host[:port], lower case, no
// path or trailing slash, and the default port (443 for https, 80 for http) left out.
func publicOrigin(u *url.URL) string {
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if u.Scheme == "https" && port == "443" || u.Scheme == "http" && port == "80" {
		port = ""
	}
	switch {
	case port != "":
		host = net.JoinHostPort(host, port)
	case strings.Contains(host, ":"):
		host = "[" + host + "]"
	}
	return u.Scheme + "://" + host
}

// bindErrorHandler is the generated router's ErrorHandlerFunc: a path or query parameter that
// does not bind is a 422 validation problem. The error is never logged or sent: it quotes the
// value.
func bindErrorHandler(w http.ResponseWriter, r *http.Request, _ error) {
	slog.WarnContext(r.Context(), "parameter rejected",
		"code", "param_invalid", "path", r.URL.Path, "request_id", logging.RequestID(r.Context()))
	problem.Write(w, r, problem.Validation{})
}

// decodeJSON decodes the request body into v: at most 1 MB, no unknown fields, nothing after the
// value. On failure it writes the problem (413 payload-too-large or 422 validation) and returns
// it; the handler then returns. The decode error is never logged: it can quote the body.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil {
		// A second value, even an empty one, means trailing data.
		if extra := dec.Decode(&json.RawMessage{}); !errors.Is(extra, io.EOF) {
			err = fmt.Errorf("trailing data after the body: %w", extra)
		}
	}
	if err == nil {
		return nil
	}
	return rejectBody(w, r, err)
}

// decodePatch is decodeJSON for a PATCH body, plus one rule: a top-level field sent as null is
// 422 validation on that field, unless it is in nullable (the spec's "may be null to clear").
// The generated patch fields are pointers, and a null would otherwise decode as "not sent".
func decodePatch(w http.ResponseWriter, r *http.Request, v any, nullable ...string) error {
	var raw json.RawMessage
	if err := decodeJSON(w, r, &raw); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return rejectBody(w, r, err)
	}
	// v decoded as a struct, so raw is a JSON object or a bare null (a nil map), which is refused.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return rejectBody(w, r, fmt.Errorf("patch body is not an object: %w", err))
	}
	var nulls []problem.FieldError
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		if string(fields[name]) == "null" && !slices.Contains(nullable, name) {
			nulls = append(nulls, problem.FieldError{Field: name, Message: problem.MsgNullNotAllowed})
		}
	}
	if len(nulls) == 0 {
		return nil
	}
	slog.WarnContext(r.Context(), "body rejected",
		"code", "null_field", "path", r.URL.Path, "request_id", logging.RequestID(r.Context()))
	p := problem.Validation{Errors: nulls}
	problem.Write(w, r, p)
	return p
}

// rejectBody writes the problem for a body that did not decode (413 when it was over the limit,
// else 422), logs its code and returns the problem. The error is never logged: it can quote the
// body.
func rejectBody(w http.ResponseWriter, r *http.Request, err error) error {
	var p error = problem.Validation{}
	code := "body_invalid"
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		p, code = problem.PayloadTooLarge{}, "payload_too_large"
	}
	slog.WarnContext(r.Context(), "body rejected",
		"code", code, "path", r.URL.Path, "request_id", logging.RequestID(r.Context()))
	problem.Write(w, r, p)
	return p
}

// writeError writes err as a problem. problem.Write logs an internal error's text, and a Postgres
// message or detail can quote input (a unique violation names the key, an invalid-input error
// quotes the value). So a Postgres error in the chain is first replaced, in place, by its SQLSTATE,
// constraint and table: the log keeps the wrap path (our own fixed prefixes) and never the message
// or detail. A unique violation or a deadlock is then a plain 500 internal.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		redacted := fmt.Sprintf("database error (SQLSTATE %s, constraint %q, table %q)",
			pgErr.Code, pgErr.ConstraintName, pgErr.TableName)
		path, found := strings.CutSuffix(err.Error(), pgErr.Error())
		if !found || pgErr.Message != "" && strings.Contains(path, pgErr.Message) {
			// The Postgres text is not a plain suffix (a joined error), or the rest quotes its
			// message: keep none of the text.
			path = ""
		}
		err = errors.New(path + redacted)
	}
	problem.Write(w, r, err)
}

// inWriteTx runs fn in a read-write transaction (db.WithTx, default options) and reports whether
// fn and the commit succeeded. On false it has written the error as a problem and the handler
// returns. A handler sets cookies and writes its response only after true, so a failed commit is
// never shown as saved.
func (s *Server) inWriteTx(w http.ResponseWriter, r *http.Request, fn func(pgx.Tx) error) bool {
	if err := db.WithTx(r.Context(), s.beginner, pgx.TxOptions{}, fn); err != nil {
		writeError(w, r, err)
		return false
	}
	return true
}

// writeJSON writes v as a JSON response with status. The status is already sent when encoding
// fails, so the failure can only be logged.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("response not written", "code", "response_write_failed")
	}
}
