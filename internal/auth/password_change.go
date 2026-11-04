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

// ChangePassword sets the signed-in user's password to next (POST /auth/password). A wrong
// current is Validation on current_password, never Unauthorized, which would sign her out; a next
// under the minimum length is Validation on new_password. The current password check goes
// through the same per-username delay as Login (item 41): the attempt is counted before the
// check and taken back when current is right, so a stolen session cannot guess the current
// password without limit, and an attempt that comes too early is RateLimited. On success it
// clears must_change_password and revokes every other session of the user, keeping session
// (item 39). A session whose user is gone is Unauthorized. The handler runs it in db.WithTx.
func (a *Auth) ChangePassword(ctx context.Context, q db.DBTX, session Session, current, next string) error {
	user, err := sqlc.New(q).GetUserCredentials(ctx, session.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem.Unauthorized{}
	}
	if err != nil {
		return fmt.Errorf("load the password hash: %w", err)
	}
	// The stored username is already normalized, so it is the key Login uses.
	if err := a.limiter.reserveName(user.Username, a.clock.Now()); err != nil {
		return err
	}
	ok, err := a.verify(user.PasswordHash, current)
	if err != nil {
		return err
	}
	if !ok {
		return problem.Validation{Errors: []problem.FieldError{{Field: "current_password", Message: problem.MsgCurrentPasswordWrong}}}
	}
	a.limiter.recordSuccess(user.Username)
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
	hash, err := hashPassword(pw)
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
