//go:build db

package auth_test

import (
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var (
	loginIP      = netip.MustParseAddr("192.0.2.10")
	otherLoginIP = netip.MustParseAddr("192.0.2.20")
)

// newLoginFixture is a setup fixture whose setup is complete: the owner "ana" exists with the
// password from validSetupInput.
func newLoginFixture(t *testing.T) (*setupFixture, auth.Me) {
	t.Helper()
	f := newSetupFixture(t)
	me, _, err := f.auth.CompleteSetup(t.Context(), f.pool, validSetupInput(f.token))
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}
	return f, me
}

func wantInvalidCredentials(t *testing.T, err error) {
	t.Helper()
	if !errors.As(err, new(problem.InvalidCredentials)) {
		t.Fatalf("err = %v; want problem.InvalidCredentials", err)
	}
}

func wantRateLimited(t *testing.T, err error) time.Duration {
	t.Helper()
	var rl problem.RateLimited
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v; want problem.RateLimited", err)
	}
	return rl.RetryAfter
}

func TestLoginOK(t *testing.T) {
	f, owner := newLoginFixture(t)
	in := auth.LoginInput{Username: " Ana ", Password: validSetupInput("").Password}

	me, token, err := f.auth.Login(t.Context(), f.pool, in, loginIP)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if want := (auth.Me{ID: owner.ID, Username: "ana"}); !reflect.DeepEqual(me, want) {
		t.Fatalf("Login me = %+v; want %+v", me, want)
	}
	got, _, _, err := f.auth.Authenticate(t.Context(), f.pool, token)
	if err != nil || got.ID != owner.ID {
		t.Fatalf("Authenticate(login token) = %+v, %v; want the owner", got, err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	f, _ := newLoginFixture(t)
	sessions := count(t, f.pool, "sessions")

	_, token, err := f.auth.Login(t.Context(), f.pool, auth.LoginInput{Username: "ana", Password: "wrong-password"}, loginIP)
	wantInvalidCredentials(t, err)
	if token != "" || count(t, f.pool, "sessions") != sessions {
		t.Fatalf("wrong password: token %q, sessions %d → %d; want no new session", token, sessions, count(t, f.pool, "sessions"))
	}
}

func TestLoginWrongPasswordDelaysTheNextAttempt(t *testing.T) {
	f, _ := newLoginFixture(t)
	_, _, err := f.auth.Login(t.Context(), f.pool, auth.LoginInput{Username: "ana", Password: "wrong-password"}, loginIP)
	wantInvalidCredentials(t, err)

	right := auth.LoginInput{Username: "ana", Password: validSetupInput("").Password}
	_, _, err = f.auth.Login(t.Context(), f.pool, right, loginIP)
	if got := wantRateLimited(t, err); got != time.Second {
		t.Fatalf("right password 0s after a failure: RetryAfter = %v; want 1s", got)
	}
}

func TestLoginSuccessClearsTheDelay(t *testing.T) {
	f, _ := newLoginFixture(t)
	wrong := auth.LoginInput{Username: "ana", Password: "wrong-password"}
	right := auth.LoginInput{Username: "ana", Password: validSetupInput("").Password}
	_, _, err := f.auth.Login(t.Context(), f.pool, wrong, loginIP)
	wantInvalidCredentials(t, err)
	f.clock.Advance(time.Second)
	if _, _, err := f.auth.Login(t.Context(), f.pool, right, loginIP); err != nil {
		t.Fatalf("right password after the wait: %v", err)
	}

	_, _, err = f.auth.Login(t.Context(), f.pool, wrong, loginIP)
	wantInvalidCredentials(t, err)
	_, _, err = f.auth.Login(t.Context(), f.pool, right, loginIP)
	if got := wantRateLimited(t, err); got != time.Second {
		t.Fatalf("first failure after a success: RetryAfter = %v; want 1s (the doubling restarted)", got)
	}
}

func TestLoginUnknownUser(t *testing.T) {
	f, _ := newLoginFixture(t)
	in := auth.LoginInput{Username: "made-up-stranger", Password: "made-up-password"}

	_, _, err := f.auth.Login(t.Context(), f.pool, in, loginIP)
	wantInvalidCredentials(t, err)
	_, _, err = f.auth.Login(t.Context(), f.pool, in, loginIP)
	if got := wantRateLimited(t, err); got != time.Second {
		t.Fatalf("unknown name again 0s later: RetryAfter = %v; want 1s (the delay applies to it too)", got)
	}
}

// OP17: an unknown username still runs one password verify, against dummyHash, so the response
// time does not reveal which usernames exist.
func TestLoginUnknownUserVerifiesAgainstTheDummyHash(t *testing.T) {
	f, _ := newLoginFixture(t)
	calls := auth.RecordVerifyCalls(f.auth)

	_, _, err := f.auth.Login(t.Context(), f.pool, auth.LoginInput{Username: "made-up-stranger", Password: "made-up-password"}, loginIP)
	wantInvalidCredentials(t, err)
	if want := []string{auth.DummyHash}; !reflect.DeepEqual(*calls, want) {
		t.Fatalf("verify calls for an unknown username = %q; want exactly one, with dummyHash", *calls)
	}
}

func TestLoginBadUsernameFormat(t *testing.T) {
	f, _ := newLoginFixture(t)
	_, _, err := f.auth.Login(t.Context(), f.pool, auth.LoginInput{Username: "a b", Password: "made-up-password"}, loginIP)
	want := problem.Validation{Errors: []problem.FieldError{{Field: "username", Message: problem.MsgUsernameInvalid}}}
	if !reflect.DeepEqual(err, want) {
		t.Fatalf("err = %#v; want %#v", err, want)
	}
}

func TestLoginRateLimitedPerIP(t *testing.T) {
	f, _ := newLoginFixture(t)
	// A distinct username per attempt keeps the per-username delay out of the way, so only the IP
	// bucket can block (controller ruling T17).
	attempt := func(i int, ip netip.Addr) error {
		in := auth.LoginInput{Username: fmt.Sprintf("made-up-%d", i), Password: "made-up-password"}
		_, _, err := f.auth.Login(t.Context(), f.pool, in, ip)
		return err
	}
	for i := range 5 {
		wantInvalidCredentials(t, attempt(i, loginIP))
	}
	if got := wantRateLimited(t, attempt(5, loginIP)); got != 12*time.Second {
		t.Fatalf("6th attempt from one IP: RetryAfter = %v; want 12s", got)
	}
	wantInvalidCredentials(t, attempt(6, otherLoginIP))
}
