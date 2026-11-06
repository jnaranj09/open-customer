package auth

import (
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// Login protection (design doc item 41); ChangePassword shares the per-username delay. Every
// time comes from the caller's injected clock.
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
	// sweepInterval: the idle-entry sweep runs at most this often, so a burst of logins does not
	// walk both maps under the lock on every call.
	sweepInterval = time.Second
)

// loginLimiter holds the per-IP buckets and the per-username delays. Idle entries are swept by
// the calls themselves, at most once per sweepInterval, so there is no goroutine and no size cap.
// It never sleeps: a blocked attempt gets problem.RateLimited with the time left.
type loginLimiter struct {
	mu        sync.Mutex
	ips       map[netip.Prefix]*ipEntry // keyed by ipBucketKey
	names     map[string]*nameEntry
	lastSweep time.Time
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
	return &loginLimiter{ips: map[netip.Prefix]*ipEntry{}, names: map[string]*nameEntry{}}
}

// ipBucketKey is the bucket ip counts against. An IPv4 address (and an IPv4-mapped IPv6 address,
// ::ffff:a.b.c.d, unmapped first) is its own bucket. An IPv6 address counts against its /64: one
// client usually holds a whole /64, and a bucket per address would let it change address on
// every attempt. An address that is not valid (the zero netip.Addr of a peer that did not parse)
// gets the zero Prefix, one bucket shared by all of them, never no limit.
func ipBucketKey(ip netip.Addr) netip.Prefix {
	ip = ip.Unmap().WithZone("")
	bits := 64
	if ip.Is4() {
		bits = 32
	}
	key, err := ip.Prefix(bits)
	if err != nil {
		return netip.Prefix{}
	}
	return key
}

// allowIP takes one attempt from the bucket of ip (ipBucketKey), or is problem.RateLimited until
// a token refills.
func (l *loginLimiter) allowIP(ip netip.Addr, now time.Time) error {
	key := ipBucketKey(ip)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	e, ok := l.ips[key]
	if !ok {
		e = &ipEntry{limiter: rate.NewLimiter(rate.Every(ipRefill), ipBurst)}
		l.ips[key] = e
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

// reserveName is problem.RateLimited while name is still waiting out the delay its last failure
// set. Otherwise it counts a failure for name at now and returns nil: the attempt is assumed to
// fail until recordSuccess takes it back. Checking and counting under one lock means two logins
// for one name that arrive together cannot both pass before the slow password check.
func (l *loginLimiter) reserveName(name string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	e, ok := l.names[name]
	// The sweep may have been skipped, so an expired entry can still be here; it counts as gone.
	if !ok || nameEntryExpired(e, now) {
		e = &nameEntry{}
		l.names[name] = e
	}
	if e.failures > 0 {
		if until := e.lastFailure.Add(nameDelay(e.failures)); now.Before(until) {
			return problem.RateLimited{RetryAfter: until.Sub(now)}
		}
	}
	e.failures++
	e.lastFailure = now
	return nil
}

// recordSuccess clears name's delay, including the failure its reservation counted.
func (l *loginLimiter) recordSuccess(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.names, name)
}

// sweep removes IP entries idle for ipIdleLimit and username entries whose last failure is
// nameEntryLifetime old. It does nothing if it ran less than sweepInterval ago. l.mu must be held.
func (l *loginLimiter) sweep(now time.Time) {
	if now.Before(l.lastSweep.Add(sweepInterval)) {
		return
	}
	l.lastSweep = now
	for key, e := range l.ips {
		if !now.Before(e.lastSeen.Add(ipIdleLimit)) {
			delete(l.ips, key)
		}
	}
	for name, e := range l.names {
		if nameEntryExpired(e, now) {
			delete(l.names, name)
		}
	}
}

// nameEntryExpired reports whether e's last failure is nameEntryLifetime old at now.
func nameEntryExpired(e *nameEntry, now time.Time) bool {
	return !now.Before(e.lastFailure.Add(nameEntryLifetime))
}

// nameDelay is the wait after the given number of failures: 1 s, 2 s, 4 s, … up to 30 s.
func nameDelay(failures int) time.Duration {
	d := nameFirstDelay
	for i := 1; i < failures && d < nameMaxDelay; i++ {
		d *= 2
	}
	return min(d, nameMaxDelay)
}
