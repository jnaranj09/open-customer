package auth

import (
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// Login protection (design doc item 41). Every time comes from the caller's injected clock.
const (
	// ipRefill and ipBurst are 5 attempts per minute per client IP, burst 5.
	ipRefill = 12 * time.Second
	ipBurst  = 5
	// ipIdleLimit: an IP entry not used for this long is removed.
	ipIdleLimit = 10 * time.Minute
	// nameFirstDelay is the wait after the first failure for a username; it doubles after each
	// further failure, up to nameMaxDelay.
	nameFirstDelay = time.Second
	nameMaxDelay   = 30 * time.Second
	// nameEntryLifetime: a username entry is removed this long after its last failure.
	nameEntryLifetime = 15 * time.Minute
)

// loginLimiter holds the per-IP buckets and the per-username delays. Idle entries are swept on
// each call, so there is no goroutine and no size cap. It never sleeps: a blocked attempt gets
// problem.RateLimited with the time left.
type loginLimiter struct {
	mu    sync.Mutex
	ips   map[netip.Addr]*ipEntry
	names map[string]*nameEntry
}

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type nameEntry struct {
	failures    int
	lastFailure time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{ips: map[netip.Addr]*ipEntry{}, names: map[string]*nameEntry{}}
}

// allowIP takes one attempt from ip's bucket, or is problem.RateLimited until a token refills.
func (l *loginLimiter) allowIP(ip netip.Addr, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	e, ok := l.ips[ip]
	if !ok {
		e = &ipEntry{limiter: rate.NewLimiter(rate.Every(ipRefill), ipBurst)}
		l.ips[ip] = e
	}
	e.lastSeen = now
	if e.limiter.AllowN(now, 1) {
		return nil
	}
	// The bucket refills one token per ipRefill; the wait is the missing fraction of one token.
	// Rounding drops the float noise from TokensAt.
	missing := 1 - e.limiter.TokensAt(now)
	return problem.RateLimited{RetryAfter: time.Duration(missing * float64(ipRefill)).Round(time.Millisecond)}
}

// checkName is problem.RateLimited while name is still waiting out the delay its last failure set.
func (l *loginLimiter) checkName(name string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	e, ok := l.names[name]
	if !ok {
		return nil
	}
	until := e.lastFailure.Add(nameDelay(e.failures))
	if now.Before(until) {
		return problem.RateLimited{RetryAfter: until.Sub(now)}
	}
	return nil
}

// recordFailure counts a failed login for name at now.
func (l *loginLimiter) recordFailure(name string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	e, ok := l.names[name]
	if !ok {
		e = &nameEntry{}
		l.names[name] = e
	}
	e.failures++
	e.lastFailure = now
}

// recordSuccess clears name's delay.
func (l *loginLimiter) recordSuccess(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.names, name)
}

// sweep removes IP entries idle for ipIdleLimit and username entries whose last failure is
// nameEntryLifetime old. l.mu must be held.
func (l *loginLimiter) sweep(now time.Time) {
	for ip, e := range l.ips {
		if !now.Before(e.lastSeen.Add(ipIdleLimit)) {
			delete(l.ips, ip)
		}
	}
	for name, e := range l.names {
		if !now.Before(e.lastFailure.Add(nameEntryLifetime)) {
			delete(l.names, name)
		}
	}
}

// nameDelay is the wait after the given number of failures: 1 s, 2 s, 4 s, … up to 30 s.
func nameDelay(failures int) time.Duration {
	d := nameFirstDelay
	for i := 1; i < failures && d < nameMaxDelay; i++ {
		d *= 2
	}
	return min(d, nameMaxDelay)
}
