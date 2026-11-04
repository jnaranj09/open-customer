package auth

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// dummyHash is verified when the username is unknown, so a login for a name that does not exist
// takes as long as one with a wrong password and the response time does not reveal which
// usernames exist (OP17). It uses defaultParams; its salt and key are random bytes that belong to
// no password.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=1$SXsryKuxNHN3aF6ZEJscfg$1v7YGZZ+8HKjLCigjoz+WoK7sWwTYbBE0eNBOJJaTW4"

// LoginInput is the POST /auth/login body.
type LoginInput struct {
	Username, Password string
}

// Login checks the username and password and creates a session. ip is the client IP the caller
// derived (httpapi decides whether to trust CF-Connecting-IP). In order: a malformed username is
// Validation (422); a used-up IP bucket or a username still waiting out its delay is RateLimited
// (429); otherwise the attempt counts as a failure for that username before the password is
// checked, so parallel attempts cannot skip the delay. An unknown username or a wrong password is
// then InvalidCredentials (401) and the failure stands; success takes it back and clears the
// username's delay. It returns the user and the session token for the cookie. The handler runs it
// in db.WithTx.
func (a *Auth) Login(ctx context.Context, q db.DBTX, in LoginInput, ip netip.Addr) (Me, string, error) {
	username, err := normalizeUsername(in.Username)
	if err != nil {
		return Me{}, "", err
	}
	if err := a.limiter.allowIP(ip, a.clock.Now()); err != nil {
		return Me{}, "", err
	}
	if err := a.limiter.reserveName(username, a.clock.Now()); err != nil {
		return Me{}, "", err
	}
	user, found, err := loadUserByName(ctx, q, username)
	if err != nil {
		return Me{}, "", err
	}
	stored := dummyHash
	if found {
		stored = user.PasswordHash
	}
	ok, err := verifyPassword(stored, in.Password)
	if err != nil {
		return Me{}, "", err
	}
	if !found || !ok {
		return Me{}, "", problem.InvalidCredentials{}
	}
	a.limiter.recordSuccess(username)
	token, err := a.createSession(ctx, q, user.ID)
	if err != nil {
		return Me{}, "", err
	}
	return Me{ID: user.ID, Username: user.Username, MustChangePassword: user.MustChangePassword}, token, nil
}

// loadUserByName loads the user named username (Login and ResetPassword); found is false when
// there is none.
func loadUserByName(ctx context.Context, q db.DBTX, username string) (sqlc.GetUserByUsernameRow, bool, error) {
	user, err := sqlc.New(q).GetUserByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.GetUserByUsernameRow{}, false, nil
	}
	if err != nil {
		return sqlc.GetUserByUsernameRow{}, false, fmt.Errorf("load a user by username: %w", err)
	}
	return user, true, nil
}
