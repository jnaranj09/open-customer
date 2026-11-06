package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	// tempPasswordAlphabet is a–z, A–Z and 2–9 without the look-alikes 0, O, 1, l and I (item 19).
	tempPasswordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	tempPasswordLength   = 12
)

// PasswordChangeAttempt is a password change that passed the per-username delay. Only
// BeginPasswordChange makes one; ChangePassword refuses the zero value (errAttemptNotBegun).
type PasswordChangeAttempt struct {
	username string
}

// BeginPasswordChange puts the current password check of me (the signed-in user step 6 loaded)
// through the same per-username delay as Login (item 41), in memory and before the handler opens
// its transaction: an attempt that comes too early is RateLimited and holds no database
// connection. Otherwise the attempt is counted before the check and ChangePassword takes it back
// when current is right, so a stolen session cannot guess the current password without limit.
// me.Username is already normalized, so it is the key Login uses.
func (a *Auth) BeginPasswordChange(me Me) (PasswordChangeAttempt, error) {
	if err := a.limiter.reserveName(me.Username, a.clock.Now()); err != nil {
		return PasswordChangeAttempt{}, err
	}
	return PasswordChangeAttempt{username: me.Username}, nil
}

// ChangePassword sets the signed-in user's password to next (POST /auth/password), for an
// attempt BeginPasswordChange let through. A wrong current is Validation on current_password,
// never Unauthorized, which would sign her out, and the failure BeginPasswordChange counted
// stands; a next under the minimum length is Validation on new_password. On success it clears
// the delay and must_change_password and revokes every other session of the user, keeping
// session (item 39). A session whose user is gone is Unauthorized; an attempt begun for another
// user fails closed (errAttemptNotBegun). The handler runs it in db.WithTx.
func (a *Auth) ChangePassword(ctx context.Context, q db.DBTX, session Session, attempt PasswordChangeAttempt, current, next string) error {
	user, err := sqlc.New(q).GetUserCredentials(ctx, session.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem.Unauthorized{}
	}
	if err != nil {
		return fmt.Errorf("load the password hash: %w", err)
	}
	if user.Username != attempt.username {
		// The zero attempt (the delay skipped), or one begun for someone else: clearing the delay
		// below would clear the wrong name. A stored username is never empty.
		return fmt.Errorf("%w: not begun for this user", errAttemptNotBegun)
	}
	ok, err := a.verify(ctx, user.PasswordHash, current)
	if err != nil {
		return err
	}
	if !ok {
		return problem.Field("current_password", problem.MsgCurrentPasswordWrong)
	}
	a.limiter.recordSuccess(attempt.username)
	if err := validatePassword("new_password", next); err != nil {
		return err
	}
	if err := setPassword(ctx, q, session.UserID, next, false); err != nil {
		return err
	}
	return revokeOtherSessions(ctx, q, session)
}

// ResetPassword gives the user a random temporary password, sets must_change_password and
// revokes all of the user's sessions (the reset-password subcommand, item 19). The username is
// normalized exactly like Login, so a malformed one is Validation; an unknown one is NotFound.
// It returns the temporary password for the caller to print; it is never logged.
func (a *Auth) ResetPassword(ctx context.Context, q db.DBTX, username string) (string, error) {
	name, err := normalizeUsername(username)
	if err != nil {
		return "", err
	}
	user, found, err := loadUserByName(ctx, q, name)
	if err != nil {
		return "", err
	}
	if !found {
		return "", problem.NotFound{}
	}
	temp, err := tempPassword()
	if err != nil {
		return "", err
	}
	if err := setPassword(ctx, q, user.ID, temp, true); err != nil {
		return "", err
	}
	if err := revokeAllSessions(ctx, q, user.ID); err != nil {
		return "", err
	}
	return temp, nil
}

// setPassword stores pw hashed for the user, with the given must_change_password flag.
func setPassword(ctx context.Context, q db.DBTX, userID uuid.UUID, pw string, mustChange bool) error {
	hash, err := hashPassword(ctx, pw)
	if err != nil {
		return err
	}
	if err := sqlc.New(q).SetUserPassword(ctx, sqlc.SetUserPasswordParams{
		PasswordHash: hash, MustChangePassword: mustChange, ID: userID,
	}); err != nil {
		return fmt.Errorf("set the password: %w", err)
	}
	return nil
}

// tempPassword draws tempPasswordLength characters from tempPasswordAlphabet with crypto/rand.
// rand.Int is uniform over [0, n) (it rejects out-of-range draws), so there is no modulo bias.
func tempPassword() (string, error) {
	n := big.NewInt(int64(len(tempPasswordAlphabet)))
	out := make([]byte, tempPasswordLength)
	for i := range out {
		idx, err := rand.Int(rand.Reader, n)
		if err != nil {
			return "", fmt.Errorf("draw a temporary password: %w", err)
		}
		out[i] = tempPasswordAlphabet[idx.Int64()]
	}
	return string(out), nil
}
