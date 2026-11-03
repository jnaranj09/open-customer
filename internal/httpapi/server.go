package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
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
	Status         *status.Status
	Log            *slog.Logger
}

// Server implements gen.ServerInterface over the module values (design doc item 32).
type Server struct {
	pool   *pgxpool.Pool
	auth   *auth.Auth
	status *status.Status
}

var _ gen.ServerInterface = (*Server)(nil)

// New returns the app's whole HTTP handler: the probes, the generated /api/v1 routes, the /api/
// catch-all and the SPA, inside the middleware steps of design doc item 34.
func New(opts Options) http.Handler {
	s := &Server{pool: opts.Pool, auth: opts.Auth, status: opts.Status}
	mux := http.NewServeMux()
	// The probes are outside /api/v1 and the spec, so they are registered by literal path.
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	gen.HandlerWithOptions(s, gen.StdHTTPServerOptions{
		BaseURL:          apiBase,
		BaseRouter:       mux,
		ErrorHandlerFunc: bindErrorHandler,
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
	})
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

// writeJSON writes v as a JSON response with status. The status is already sent when encoding
// fails, so the failure can only be logged.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("response not written", "code", "response_write_failed")
	}
}
