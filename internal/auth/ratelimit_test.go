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

func TestUsernameDelayDoubles(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	at := limitStart
	for _, want := range []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second} {
		if err := l.checkName("made-up-name", at); err != nil {
			t.Fatalf("attempt at +%v: %v; want nil", at.Sub(limitStart), err)
		}
		l.recordFailure("made-up-name", at)
		if got := retryAfter(t, l.checkName("made-up-name", at)); got != want {
			t.Fatalf("right after the failure at +%v: RetryAfter = %v; want %v", at.Sub(limitStart), got, want)
		}
		at = at.Add(want)
	}
}

func TestUsernameDelayCappedAtThirtySeconds(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	at := limitStart
	for range 10 {
		l.recordFailure("made-up-name", at)
		at = at.Add(time.Minute)
	}
	last := at.Add(-time.Minute)
	if got := retryAfter(t, l.checkName("made-up-name", last)); got != 30*time.Second {
		t.Fatalf("after 10 failures: RetryAfter = %v; want 30s", got)
	}
}

func TestUsernameDelayResetsOnSuccess(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	l.recordFailure("made-up-name", limitStart)
	l.recordFailure("made-up-name", limitStart.Add(time.Second))
	l.recordSuccess("made-up-name")
	if err := l.checkName("made-up-name", limitStart.Add(time.Second)); err != nil {
		t.Fatalf("after a success: %v; want nil", err)
	}
	l.recordFailure("made-up-name", limitStart.Add(time.Second))
	if got := retryAfter(t, l.checkName("made-up-name", limitStart.Add(time.Second))); got != time.Second {
		t.Fatalf("first failure after a success: RetryAfter = %v; want 1s (the doubling restarts)", got)
	}
}

func TestUsernameEntryExpiresAfterFifteenMinutes(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	for i := range 6 {
		l.recordFailure("made-up-name", limitStart.Add(time.Duration(i)*time.Minute))
	}
	last := limitStart.Add(5 * time.Minute)
	if err := l.checkName("made-up-name", last.Add(15*time.Minute-time.Second)); err != nil {
		t.Fatalf("14m59s after the last failure: %v; want nil (the 30s wait is over)", err)
	}
	if _, kept := l.names["made-up-name"]; !kept {
		t.Fatal("14m59s after the last failure: the entry is gone; want it kept")
	}
	if err := l.checkName("other-name", last.Add(15*time.Minute)); err != nil {
		t.Fatalf("another name: %v", err)
	}
	if _, kept := l.names["made-up-name"]; kept {
		t.Fatal("15m after the last failure: the entry is kept; want it removed")
	}
	l.recordFailure("made-up-name", last.Add(15*time.Minute))
	if got := retryAfter(t, l.checkName("made-up-name", last.Add(15*time.Minute))); got != time.Second {
		t.Fatalf("first failure after expiry: RetryAfter = %v; want 1s", got)
	}
}

func TestTooEarlyAttemptReturnsRetryAfter(t *testing.T) {
	t.Parallel()
	l := newLoginLimiter()
	l.recordFailure("made-up-name", limitStart)
	l.recordFailure("made-up-name", limitStart.Add(time.Second))
	// The second failure asks for 2s; 500ms later 1.5s remain.
	if got := retryAfter(t, l.checkName("made-up-name", limitStart.Add(1500*time.Millisecond))); got != 1500*time.Millisecond {
		t.Fatalf("RetryAfter = %v; want 1.5s (what remains of the wait)", got)
	}
	if err := l.checkName("made-up-name", limitStart.Add(3*time.Second)); err != nil {
		t.Fatalf("once the wait is over: %v; want nil", err)
	}
}
