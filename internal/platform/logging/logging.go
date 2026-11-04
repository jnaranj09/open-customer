// Package logging is the JSON slog setup and the request-log middleware.
package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// New returns a JSON logger at level.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

type requestIDKey struct{}

// RequestID returns the ID that Middleware put on ctx, or "" outside a request.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

type routeKey struct{}

// routeHolder carries the matched pattern back out to Middleware. The mux sets Pattern only on the
// request it is given, and a layer between Middleware and the mux that calls r.WithContext hands
// it a copy, so Middleware's own r.Pattern stays empty.
type routeHolder struct{ pattern string }

// SetRoute records the matched route pattern for the request log line. The layer just outside the
// mux calls it after routing. Outside a request it does nothing.
func SetRoute(ctx context.Context, pattern string) {
	if h, ok := ctx.Value(routeKey{}).(*routeHolder); ok {
		h.pattern = pattern
	}
}

// Middleware gives each request an ID and writes one log line after it: request ID, cf_ray,
// route pattern, status and duration. The route is the one SetRoute recorded, or r.Pattern when
// the mux sits directly inside Middleware.
func Middleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		route := &routeHolder{}
		ctx := context.WithValue(r.Context(), requestIDKey{}, newID())
		r = r.WithContext(context.WithValue(ctx, routeKey{}, route))
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if route.pattern == "" {
			route.pattern = r.Pattern
		}
		log.LogAttrs(r.Context(), slog.LevelInfo, "request",
			slog.String("request_id", RequestID(r.Context())),
			slog.String("cf_ray", r.Header.Get("CF-Ray")),
			slog.String("method", r.Method),
			slog.String("route", route.pattern),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error (Go 1.24+).
	return hex.EncodeToString(b[:])
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status, s.wroteHeader = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the real writer (flush, deadlines).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
