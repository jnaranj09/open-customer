// Package httpapi is the HTTP layer: the middleware chain, the handlers behind the generated
// server interface, and the SPA serving (design doc items 32–37).
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"runtime/debug"

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
