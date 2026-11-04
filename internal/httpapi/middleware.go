// Package httpapi is the HTTP layer: the middleware chain, the handlers behind the generated
// server interface, and the SPA serving (design doc items 32–37).
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"runtime/debug"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// middlewareConfig is what the middleware steps 1–5 (design doc item 34) need from serve.
type middlewareConfig struct {
	Version string // sent as X-App-Version on every response
	// PublicOrigin is PUBLIC_URL as an Origin value: scheme, host and port, no path, no trailing
	// slash. Every request except GET and HEAD must send exactly this Origin.
	PublicOrigin string
	// TrustedProxies are the peers whose CF-Connecting-IP is believed (TRUSTED_PROXY_CIDRS).
	TrustedProxies []netip.Prefix
	Log            *slog.Logger
}

// wrap applies design doc item 34 steps 1–5 around next, outermost first: request ID and log
// line, X-App-Version, panic recovery, client IP, Origin check. The innermost layer records the
// mux's route pattern for the log line.
func wrap(next http.Handler, cfg middlewareConfig) http.Handler {
	h := recordRoute(next)
	h = checkOrigin(cfg, h)
	h = withClientIP(cfg, h)
	h = recoverPanic(cfg, h)
	h = appVersion(cfg.Version, h)
	return logging.Middleware(cfg.Log, h)
}

// recordRoute passes the matched pattern back to logging.Middleware. The mux sets Pattern on the
// request it is given, which is this layer's r (earlier layers hand on r.WithContext copies, so
// the outer r never sees it). The defer also runs when the handler panics.
func recordRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { logging.SetRoute(r.Context(), r.Pattern) }()
		next.ServeHTTP(w, r)
	})
}

// appVersion sets X-App-Version before anything inside can write, so errors and panics carry it.
func appVersion(version string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-App-Version", version)
		next.ServeHTTP(w, r)
	})
}

// errPanic is what problem.Write gets for a recovered panic: not a problem value, so the client
// sees only "internal". The panic value itself is never logged or sent (it can hold input).
var errPanic = errors.New("handler panicked")

func recoverPanic(cfg middlewareConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() == nil {
				return
			}
			cfg.Log.ErrorContext(r.Context(), "panic",
				"code", "panic", "request_id", logging.RequestID(r.Context()), "stack", string(debug.Stack()))
			problem.Write(w, r, errPanic)
		}()
		next.ServeHTTP(w, r)
	})
}

type clientIPKey struct{}

// clientIP returns the address withClientIP stored on ctx (the zero Addr outside a request).
func clientIP(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(clientIPKey{}).(netip.Addr)
	return ip
}

// withClientIP stores the client address: CF-Connecting-IP when the TCP peer is a trusted proxy,
// otherwise the peer.
func withClientIP(cfg middlewareConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := peerAddr(cfg.Log, r)
		if header := r.Header.Get("CF-Connecting-IP"); header != "" && trusted(cfg.TrustedProxies, ip) {
			if forwarded, err := netip.ParseAddr(header); err == nil {
				ip = forwarded.Unmap()
			} else {
				cfg.Log.WarnContext(r.Context(), "client IP header rejected",
					"code", "client_ip_invalid", "request_id", logging.RequestID(r.Context()))
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip)))
	})
}

// peerAddr parses r.RemoteAddr ("ip:port" from net/http). An unparsable value gives the zero
// Addr, which no trusted prefix contains.
func peerAddr(log *slog.Logger, r *http.Request) netip.Addr {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		log.WarnContext(r.Context(), "peer address rejected",
			"code", "peer_ip_invalid", "request_id", logging.RequestID(r.Context()))
		return netip.Addr{}
	}
	return ap.Addr().Unmap()
}

func trusted(prefixes []netip.Prefix, ip netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// checkOrigin refuses every method except GET and HEAD (the mux routes HEAD to GET handlers)
// unless Origin equals the public origin exactly. A missing Origin is refused too.
func checkOrigin(cfg middlewareConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin == "" || origin != cfg.PublicOrigin {
				cfg.Log.WarnContext(r.Context(), "origin rejected",
					"code", "forbidden_origin", "request_id", logging.RequestID(r.Context()))
				problem.Write(w, r, problem.ForbiddenOrigin{})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Steps 6–8 (design doc item 34) run as the generated router's middlewares (OP5), so they run
// after routing and only on the /api/v1 routes of the spec. The public routes and the routes
// allowed while must_change_password is set come from the spec, never from a route list here.

// signedIn is what step 6 stores for a valid session cookie.
type signedIn struct {
	me      auth.Me
	session auth.Session
}

type signedInKey struct{}

// currentSession returns the session step 6 loaded; ok is false when the request has none.
func currentSession(ctx context.Context) (signedIn, bool) {
	cur, ok := ctx.Value(signedInKey{}).(signedIn)
	return cur, ok
}

// sessionMiddlewares are steps 6–8 as gen.StdHTTPServerOptions.Middlewares. The generated wrapper
// wraps the handler in list order, so the last entry runs first: 6, then 7, then 8.
func (s *Server) sessionMiddlewares(allowedWhileMustChange map[string]bool) []gen.MiddlewareFunc {
	return []gen.MiddlewareFunc{mustChangePasswordGate(allowedWhileMustChange), requireSession, s.loadSession}
}

// loadSession is step 6. With an oc_session cookie it runs auth.Authenticate in a short write
// transaction (the hourly sliding write), and on success stores the session on the context,
// re-sending the cookie when the session slid. A malformed, unknown or expired cookie leaves the
// request without a session; step 7 refuses it where one is needed. Any other error (the
// database) is a 500.
func (s *Server) loadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.SessionToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		var cur signedIn
		var refreshed bool
		err := db.WithTx(r.Context(), s.beginner, pgx.TxOptions{}, func(tx pgx.Tx) error {
			var err error
			if cur.me, cur.session, refreshed, err = s.auth.Authenticate(r.Context(), tx, token); err != nil {
				return fmt.Errorf("authenticate: %w", err)
			}
			return nil
		})
		if errors.As(err, new(problem.Unauthorized)) {
			next.ServeHTTP(w, r)
			return
		}
		if err != nil {
			writeError(w, r, err)
			return
		}
		if refreshed {
			http.SetCookie(w, auth.SessionCookie(token, s.secureCookies))
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), signedInKey{}, cur)))
	})
}

// requireSession is step 7. The generated wrapper puts gen.SessionCookieScopes on the context of
// every operation under the spec's global security; the public ones say `security: []` and get
// none. A secured route without a session is 401 unauthorized.
func requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(gen.SessionCookieScopes) != nil {
			if _, ok := currentSession(r.Context()); !ok {
				problem.Write(w, r, problem.Unauthorized{})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// mustChangePasswordGate is step 8. While the signed-in user must change her password, only the
// routes in allowed (from allowedWhileMustChangePassword) pass; every other one is 403
// must-change-password. allowed is keyed by the mux pattern, as r.Pattern holds it.
func mustChangePasswordGate(allowed map[string]bool) gen.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cur, ok := currentSession(r.Context()); ok && cur.me.MustChangePassword && !allowed[r.Pattern] {
				problem.Write(w, r, problem.MustChangePassword{})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// mustChangeExtension marks, in api/openapi.yaml, the operations allowed while
// must_change_password is set.
const mustChangeExtension = "x-oc-allowed-while-must-change-password"

// allowedWhileMustChangePassword reads the operations spec marks with mustChangeExtension: true,
// as the mux patterns "METHOD /api/v1<path>" the generated router registers. A value other than a
// boolean is an error, so a typo in the spec stops serve instead of opening or closing a route.
func allowedWhileMustChangePassword(spec *openapi3.T) (map[string]bool, error) {
	allowed := map[string]bool{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			raw, found := op.Extensions[mustChangeExtension]
			if !found {
				continue
			}
			marked, isBool := raw.(bool)
			if !isBool {
				return nil, fmt.Errorf("httpapi: %s on %s %s is not a boolean", mustChangeExtension, method, path)
			}
			if marked {
				allowed[method+" "+apiBase+path] = true
			}
		}
	}
	return allowed, nil
}
