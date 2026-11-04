//go:build db

package auth

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	madeUpPassword    = "made-up-password"
	madeUpNewPassword = "made-up-new-password"
	// lookAlikes are the characters a temporary password never contains.
	lookAlikes = "0O1lI"
)

// passwordFixture is a session fixture whose user has the password madeUpPassword and must not
// change it, plus the loaded Session for the fixture's token.
type passwordFixture struct {
	sessionFixture
	session Session
}

func newPasswordFixture(t *testing.T) passwordFixture {
	t.Helper()
	f := newSessionFixture(t)
	hash, err := hashPassword(madeUpPassword)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if _, err := f.pool.Exec(t.Context(),
		`UPDATE users SET password_hash = $1, must_change_password = false WHERE id = $2`, hash, f.userID); err != nil {
		t.Fatalf("set the password: %v", err)
	}
	_, sess, _, err := f.auth.Authenticate(t.Context(), f.pool, f.token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	return passwordFixture{sessionFixture: f, session: sess}
}

// storedUser reads the user's password hash and must_change_password flag from the table.
func (f passwordFixture) storedUser(t *testing.T) (hash string, mustChange bool) {
	t.Helper()
	if err := f.pool.QueryRow(t.Context(),
		`SELECT password_hash, must_change_password FROM users WHERE id = $1`, f.userID).
		Scan(&hash, &mustChange); err != nil {
		t.Fatalf("read the user: %v", err)
	}
	return hash, mustChange
}

// wantPasswordIs fails unless pw verifies against the stored hash.
func (f passwordFixture) wantPasswordIs(t *testing.T, pw string) {
	t.Helper()
	hash, _ := f.storedUser(t)
	ok, err := verifyPassword(hash, pw)
	if err != nil || !ok {
		t.Fatalf("verifyPassword(stored, %q) = %v, %v; want true, nil", pw, ok, err)
	}
}

func (f passwordFixture) setMustChange(t *testing.T) {
	t.Helper()
	if _, err := f.pool.Exec(t.Context(),
		`UPDATE users SET must_change_password = true WHERE id = $1`, f.userID); err != nil {
		t.Fatalf("set must_change_password: %v", err)
	}
}

func wantValidationOn(t *testing.T, err error, field, message string) {
	t.Helper()
	want := problem.Validation{Errors: []problem.FieldError{{Field: field, Message: message}}}
	var got problem.Validation
	if !errors.As(err, &got) || !reflect.DeepEqual(got, want) {
		t.Fatalf("err = %#v; want %#v", err, want)
	}
}

func TestChangePasswordKeepsCurrentRevokesOthers(t *testing.T) {
	f := newPasswordFixture(t)
	others := make([]string, 2)
	for i := range others {
		token, err := f.auth.createSession(t.Context(), f.pool, f.userID)
		if err != nil {
			t.Fatalf("createSession: %v", err)
		}
		others[i] = token
	}
	if err := f.auth.ChangePassword(t.Context(), f.pool, f.session, madeUpPassword, madeUpNewPassword); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, _, _, err := f.auth.Authenticate(t.Context(), f.pool, f.token); err != nil {
		t.Fatalf("Authenticate(current session) = %v; want nil", err)
	}
	for _, token := range others {
		_, _, _, err := f.auth.Authenticate(t.Context(), f.pool, token)
		wantUnauthorized(t, err)
	}
	f.wantPasswordIs(t, madeUpNewPassword)
}

func TestChangePasswordWrongCurrentIsValidation(t *testing.T) {
	f := newPasswordFixture(t)
	err := f.auth.ChangePassword(t.Context(), f.pool, f.session, "made-up-wrong-password", madeUpNewPassword)
	if errors.As(err, new(problem.Unauthorized)) {
		t.Fatalf("err = %v; want Validation, not Unauthorized", err)
	}
	wantValidationOn(t, err, "current_password", problem.MsgCurrentPasswordWrong)
	f.wantPasswordIs(t, madeUpPassword)
}

func TestChangePasswordWrongCurrentDelaysTheUsername(t *testing.T) {
	f := newPasswordFixture(t)
	wrong := f.auth.ChangePassword(t.Context(), f.pool, f.session, "made-up-wrong-password", madeUpNewPassword)
	wantValidationOn(t, wrong, "current_password", problem.MsgCurrentPasswordWrong)

	t.Run("the next change within a second is rate limited, even with the right password", func(t *testing.T) {
		err := f.auth.ChangePassword(t.Context(), f.pool, f.session, madeUpPassword, madeUpNewPassword)
		if got := retryAfter(t, err); got != time.Second {
			t.Fatalf("RetryAfter = %v; want 1s", got)
		}
		f.wantPasswordIs(t, madeUpPassword)
	})
	t.Run("a login for the same username shares the delay", func(t *testing.T) {
		_, _, err := f.auth.Login(t.Context(), f.pool,
			LoginInput{Username: "made-up-owner", Password: madeUpPassword}, ipA)
		retryAfter(t, err)
	})
}

func TestChangePasswordSuccessClearsTheDelay(t *testing.T) {
	f := newPasswordFixture(t)
	wrong := f.auth.ChangePassword(t.Context(), f.pool, f.session, "made-up-wrong-password", madeUpNewPassword)
	wantValidationOn(t, wrong, "current_password", problem.MsgCurrentPasswordWrong)
	f.clock.Advance(time.Second)
	if err := f.auth.ChangePassword(t.Context(), f.pool, f.session, madeUpPassword, madeUpNewPassword); err != nil {
		t.Fatalf("ChangePassword after the delay: %v", err)
	}
	err := f.auth.ChangePassword(t.Context(), f.pool, f.session, "made-up-wrong-password", madeUpPassword)
	wantValidationOn(t, err, "current_password", problem.MsgCurrentPasswordWrong)
}

func TestChangePasswordTooShort(t *testing.T) {
	f := newPasswordFixture(t)
	err := f.auth.ChangePassword(t.Context(), f.pool, f.session, madeUpPassword, "short")
	wantValidationOn(t, err, "new_password", problem.MsgPasswordTooShort)
	f.wantPasswordIs(t, madeUpPassword)
}

func TestChangePasswordClearsMustChange(t *testing.T) {
	f := newPasswordFixture(t)
	f.setMustChange(t)
	if err := f.auth.ChangePassword(t.Context(), f.pool, f.session, madeUpPassword, madeUpNewPassword); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, mustChange := f.storedUser(t); mustChange {
		t.Fatal("must_change_password = true after ChangePassword; want false")
	}
}

func TestChangePasswordForAGoneUserIsUnauthorized(t *testing.T) {
	f := newPasswordFixture(t)
	gone := f.session
	gone.UserID = uuid.New()
	err := f.auth.ChangePassword(t.Context(), f.pool, gone, madeUpPassword, madeUpNewPassword)
	wantUnauthorized(t, err)
}

func TestChangePasswordCorruptStoredHashFailsClosed(t *testing.T) {
	f := newPasswordFixture(t)
	if _, err := f.pool.Exec(t.Context(),
		`UPDATE users SET password_hash = 'not-a-phc-string' WHERE id = $1`, f.userID); err != nil {
		t.Fatalf("corrupt the hash: %v", err)
	}
	err := f.auth.ChangePassword(t.Context(), f.pool, f.session, madeUpPassword, madeUpNewPassword)
	if !errors.Is(err, errMalformedHash) {
		t.Fatalf("err = %v; want errMalformedHash (a 500, not a change and not a Validation)", err)
	}
	if hash, _ := f.storedUser(t); hash != "not-a-phc-string" {
		t.Fatal("the stored hash changed; want it untouched")
	}
}

func TestResetPasswordSetsMustChangeAndRevokesAll(t *testing.T) {
	f := newPasswordFixture(t)
	other, err := f.auth.createSession(t.Context(), f.pool, f.userID)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	temp, err := f.auth.ResetPassword(t.Context(), f.pool, "made-up-owner")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	for _, token := range []string{f.token, other} {
		_, _, _, err := f.auth.Authenticate(t.Context(), f.pool, token)
		wantUnauthorized(t, err)
	}
	if _, mustChange := f.storedUser(t); !mustChange {
		t.Fatal("must_change_password = false after ResetPassword; want true")
	}
	f.wantPasswordIs(t, temp)
}

func TestResetPasswordNormalizesUsernameLikeLogin(t *testing.T) {
	f := newPasswordFixture(t)
	temp, err := f.auth.ResetPassword(t.Context(), f.pool, " Made-Up-Owner ")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	f.wantPasswordIs(t, temp)
}

func TestResetPasswordMalformedUsernameIsValidation(t *testing.T) {
	f := newPasswordFixture(t)
	_, err := f.auth.ResetPassword(t.Context(), f.pool, "no")
	wantValidationOn(t, err, "username", problem.MsgUsernameInvalid)
}

func TestResetPasswordUnknownUser(t *testing.T) {
	f := newPasswordFixture(t)
	_, err := f.auth.ResetPassword(t.Context(), f.pool, "made-up-nobody")
	if !errors.As(err, new(problem.NotFound)) {
		t.Fatalf("err = %v; want problem.NotFound", err)
	}
	f.wantPasswordIs(t, madeUpPassword)
}

func TestTempPasswordAlphabetAndLength(t *testing.T) {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for range 1000 {
		pw, err := tempPassword()
		if err != nil {
			t.Fatalf("tempPassword: %v", err)
		}
		if len(pw) != 12 {
			t.Fatalf("len(%q) = %d; want 12", pw, len(pw))
		}
		if strings.ContainsAny(pw, lookAlikes) {
			t.Fatalf("%q has a look-alike character from %q", pw, lookAlikes)
		}
		if strings.Trim(pw, alphabet) != "" {
			t.Fatalf("%q has a character outside the alphabet", pw)
		}
	}
}
