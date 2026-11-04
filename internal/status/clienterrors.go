package status

import (
	"context"
	"log/slog"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// Limits of a client error (design doc "Observability"), in runes.
const (
	maxMessageRunes    = 2000
	maxStackRunes      = 8000
	maxAppVersionRunes = 50
	maxRouteRunes      = 200
)

const (
	// clientErrorBurst is the number of client errors one session may log per clientErrorWindow.
	clientErrorBurst  = 10
	clientErrorWindow = time.Minute
	// clientErrorRefill is the time one token takes to come back.
	clientErrorRefill = clientErrorWindow / clientErrorBurst
)

// ClientError is what the PWA reports about itself.
type ClientError struct {
	Message    string
	Stack      *string
	AppVersion string
	Route      string
}

// LogClientError logs e at warn with code=client_error. sessionKey is the hex of the session's
// token hash; each key may log 10 errors per minute, then the call is problem.RateLimited. A
// message, app version or route that is empty or over its limit is a problem.Validation on that
// field. A stack over its limit is cut, not rejected: a long trace is still worth the first part.
// The texts come from our own PWA and are the only way to debug it, so they are logged (a named
// exception in the maintainability rule); everything else logs a code and never a value.
func (s *Status) LogClientError(ctx context.Context, sessionKey string, e ClientError) error {
	if err := checkClientError(e); err != nil {
		return err
	}
	if err := s.limiter.allow(sessionKey, s.clock.Now()); err != nil {
		return err
	}
	attrs := []any{
		"code", "client_error", "route", e.Route, "app_version", e.AppVersion, "message", e.Message,
	}
	if e.Stack != nil {
		attrs = append(attrs, "stack", cutRunes(*e.Stack, maxStackRunes))
	}
	slog.WarnContext(ctx, "client error", attrs...)
	return nil
}

// checkClientError reports every bad field.
func checkClientError(e ClientError) error {
	var errs []problem.FieldError
	for _, f := range []struct {
		name, value string
		limit       int
	}{
		{"message", e.Message, maxMessageRunes},
		{"app_version", e.AppVersion, maxAppVersionRunes},
		{"route", e.Route, maxRouteRunes},
	} {
		if n := utf8.RuneCountInString(f.value); n == 0 || n > f.limit {
			errs = append(errs, problem.FieldError{Field: f.name, Message: problem.MsgClientErrorFieldInvalid})
		}
	}
	if len(errs) > 0 {
		return problem.Validation{Errors: errs}
	}
	return nil
}

// cutRunes returns the first limit runes of s.
func cutRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}

// clientErrorLimiter holds one token bucket per session key. Idle buckets are full again, so the
// calls drop them, at most once per clientErrorWindow: no goroutine and no size cap.
type clientErrorLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newClientErrorLimiter() *clientErrorLimiter {
	return &clientErrorLimiter{buckets: map[string]*bucket{}}
}

// allow takes one token from key's bucket at now, or is problem.RateLimited with the time until
// the next one.
func (l *clientErrorLimiter) allow(key string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !now.Before(l.lastSweep.Add(clientErrorWindow)) {
		l.lastSweep = now
		for k, b := range l.buckets {
			if !now.Before(b.lastSeen.Add(clientErrorWindow)) {
				delete(l.buckets, k)
			}
		}
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(rate.Every(clientErrorRefill), clientErrorBurst)}
		l.buckets[key] = b
	}
	b.lastSeen = now
	if b.limiter.AllowN(now, 1) {
		return nil
	}
	missing := 1 - b.limiter.TokensAt(now)
	return problem.RateLimited{RetryAfter: time.Duration(missing * float64(clientErrorRefill)).Round(time.Millisecond)}
}
