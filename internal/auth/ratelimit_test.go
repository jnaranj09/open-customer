package auth

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var limitStart = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

var (
	ipA = netip.MustParseAddr("192.0.2.10")
	ipB = netip.MustParseAddr("192.0.2.20")
)

// retryAfter returns the RetryAfter of a RateLimited err, and fails the test for any other value.
func retryAfter(t *testing.T, err error) time.Duration {
	t.Helper()
	var rl problem.RateLimited
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v; want problem.RateLimited", err)
	}
	return rl.RetryAfter
}

func TestIPLimitFiveThenBlocked(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	for i := range 5 {
		if err := l.allowIP(ipA, limitStart); err != nil {
			t.Fatalf("attempt %d in the same instant: %v; want nil", i+1, err)
		}
	}
	if got := retryAfter(t, l.allowIP(ipA, limitStart)); got != 12*time.Second {
		t.Fatalf("6th attempt in the same instant: RetryAfter = %v; want 12s", got)
	}
	if err := l.allowIP(ipB, limitStart); err != nil {
		t.Fatalf("another IP in the same instant: %v; want nil (its own bucket)", err)
	}
}

func TestIPLimitRefillsAfterTwelveSeconds(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	for range 5 {
		if err := l.allowIP(ipA, limitStart); err != nil {
			t.Fatalf("burst: %v", err)
		}
	}
	if got := retryAfter(t, l.allowIP(ipA, limitStart.Add(11*time.Second))); got != time.Second {
		t.Fatalf("after 11s: RetryAfter = %v; want 1s", got)
	}
	if err := l.allowIP(ipA, limitStart.Add(12*time.Second)); err != nil {
		t.Fatalf("after 12s: %v; want nil (one token refilled)", err)
	}
	retryAfter(t, l.allowIP(ipA, limitStart.Add(12*time.Second)))
}

func TestIPEntriesSweptAfterTenMinutes(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	if err := l.allowIP(ipA, limitStart); err != nil {
		t.Fatalf("first attempt: %v", err)
	}
	if err := l.allowIP(ipB, limitStart.Add(10*time.Minute-time.Second)); err != nil {
		t.Fatalf("other IP: %v", err)
	}
	if got := len(l.ips); got != 2 {
		t.Fatalf("idle 9m59s: %d IP entries; want 2", got)
	}
	if err := l.allowIP(ipB, limitStart.Add(10*time.Minute)); err != nil {
		t.Fatalf("other IP again: %v", err)
	}
	if _, kept := l.ips[ipA]; kept || len(l.ips) != 1 {
		t.Fatalf("idle 10m: entries %v; want only the other IP", l.ips)
	}
}

// Each failed attempt is a reserveName that returned nil: the reservation counts the failure up
// front, and only a success (recordSuccess) takes it back.

func TestUsernameDelayDoubles(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	at := limitStart
	for _, want := range []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second} {
		if err := l.reserveName("made-up-name", at); err != nil {
			t.Fatalf("attempt at +%v: %v; want nil", at.Sub(limitStart), err)
		}
		if got := retryAfter(t, l.reserveName("made-up-name", at)); got != want {
			t.Fatalf("right after the failure at +%v: RetryAfter = %v; want %v", at.Sub(limitStart), got, want)
		}
		at = at.Add(want)
	}
}

func TestUsernameDelayCappedAtThirtySeconds(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	at := limitStart
	for i := range 10 {
		if err := l.reserveName("made-up-name", at); err != nil {
			t.Fatalf("failure %d: %v", i+1, err)
		}
		at = at.Add(time.Minute)
	}
	last := at.Add(-time.Minute)
	if got := retryAfter(t, l.reserveName("made-up-name", last)); got != 30*time.Second {
		t.Fatalf("after 10 failures: RetryAfter = %v; want 30s", got)
	}
}

func TestUsernameDelayResetsOnSuccess(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	for _, at := range []time.Time{limitStart, limitStart.Add(time.Second)} {
		if err := l.reserveName("made-up-name", at); err != nil {
			t.Fatalf("attempt at +%v: %v", at.Sub(limitStart), err)
		}
	}
	l.recordSuccess("made-up-name")
	if err := l.reserveName("made-up-name", limitStart.Add(time.Second)); err != nil {
		t.Fatalf("after a success: %v; want nil", err)
	}
	if got := retryAfter(t, l.reserveName("made-up-name", limitStart.Add(time.Second))); got != time.Second {
		t.Fatalf("first failure after a success: RetryAfter = %v; want 1s (the doubling restarts)", got)
	}
}

func TestUsernameEntryExpiresAfterFifteenMinutes(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	for i := range 6 {
		if err := l.reserveName("made-up-name", limitStart.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("failure %d: %v", i+1, err)
		}
	}
	last := limitStart.Add(5 * time.Minute)
	// allowIP sweeps both maps; the two calls are a second apart, so both sweeps run.
	if err := l.allowIP(ipA, last.Add(15*time.Minute-time.Second)); err != nil {
		t.Fatalf("allowIP: %v", err)
	}
	if _, kept := l.names["made-up-name"]; !kept {
		t.Fatal("14m59s after the last failure: the entry is gone; want it kept")
	}
	if err := l.allowIP(ipA, last.Add(15*time.Minute)); err != nil {
		t.Fatalf("allowIP: %v", err)
	}
	if _, kept := l.names["made-up-name"]; kept {
		t.Fatal("15m after the last failure: the entry is kept; want it removed")
	}
	if err := l.reserveName("made-up-name", last.Add(15*time.Minute)); err != nil {
		t.Fatalf("first attempt after expiry: %v; want nil", err)
	}
	if got := retryAfter(t, l.reserveName("made-up-name", last.Add(15*time.Minute))); got != time.Second {
		t.Fatalf("first failure after expiry: RetryAfter = %v; want 1s", got)
	}
}

// The sweep runs at most once per second, so an expired entry can still be in the map; it must
// count as gone all the same.
func TestExpiredNameEntryIgnoredBeforeTheSweep(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	for _, at := range []time.Time{limitStart, limitStart.Add(time.Second), limitStart.Add(3 * time.Second)} {
		if err := l.reserveName("made-up-name", at); err != nil {
			t.Fatalf("attempt at +%v: %v", at.Sub(limitStart), err)
		}
	}
	expiry := limitStart.Add(3*time.Second + 15*time.Minute)
	if err := l.allowIP(ipA, expiry.Add(-500*time.Millisecond)); err != nil {
		t.Fatalf("allowIP: %v", err)
	}
	if err := l.reserveName("made-up-name", expiry); err != nil {
		t.Fatalf("at expiry, sweep skipped: %v; want nil", err)
	}
	if got := retryAfter(t, l.reserveName("made-up-name", expiry)); got != time.Second {
		t.Fatalf("at expiry, sweep skipped: RetryAfter = %v; want 1s (a fresh entry, not 8s)", got)
	}
}

func TestTooEarlyAttemptReturnsRetryAfter(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	for _, at := range []time.Time{limitStart, limitStart.Add(time.Second)} {
		if err := l.reserveName("made-up-name", at); err != nil {
			t.Fatalf("attempt at +%v: %v", at.Sub(limitStart), err)
		}
	}
	// The second failure asks for 2s; 500ms later 1.5s remain.
	if got := retryAfter(t, l.reserveName("made-up-name", limitStart.Add(1500*time.Millisecond))); got != 1500*time.Millisecond {
		t.Fatalf("RetryAfter = %v; want 1.5s (what remains of the wait)", got)
	}
	if err := l.reserveName("made-up-name", limitStart.Add(3*time.Second)); err != nil {
		t.Fatalf("once the wait is over: %v; want nil", err)
	}
}

// Two logins for one name that arrive together must not both pass before either has failed: the
// reservation is taken under the lock, before the slow password check.
func TestTwoAttemptsAtOnceOnOneNameGetOneThrough(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	if err := l.reserveName("made-up-name", limitStart); err != nil {
		t.Fatalf("first attempt: %v; want nil", err)
	}
	if got := retryAfter(t, l.reserveName("made-up-name", limitStart)); got != time.Second {
		t.Fatalf("second attempt in the same instant: RetryAfter = %v; want 1s", got)
	}
}

func TestIPv4MappedAddressSharesTheIPv4Bucket(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	for range 5 {
		if err := l.allowIP(ipA, limitStart); err != nil {
			t.Fatalf("burst: %v", err)
		}
	}
	mapped := netip.AddrFrom16(ipA.As16())
	if got := retryAfter(t, l.allowIP(mapped, limitStart)); got != 12*time.Second {
		t.Fatalf("%v after 5 attempts from %v: RetryAfter = %v; want 12s (one bucket)", mapped, ipA, got)
	}
}

func TestSweepRunsAtMostOncePerSecond(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	if err := l.allowIP(ipA, limitStart); err != nil {
		t.Fatalf("allowIP: %v", err)
	}
	idle := limitStart.Add(10 * time.Minute)
	if err := l.allowIP(ipB, idle.Add(-500*time.Millisecond)); err != nil {
		t.Fatalf("allowIP: %v", err)
	}
	if err := l.allowIP(ipB, idle); err != nil {
		t.Fatalf("allowIP: %v", err)
	}
	if _, kept := l.ips[ipA]; !kept {
		t.Fatal("500ms after the last sweep: the idle entry is gone; want the sweep skipped")
	}
	if err := l.allowIP(ipB, idle.Add(500*time.Millisecond)); err != nil {
		t.Fatalf("allowIP: %v", err)
	}
	if _, kept := l.ips[ipA]; kept {
		t.Fatal("1s after the last sweep: the idle entry is kept; want it swept")
	}
}
