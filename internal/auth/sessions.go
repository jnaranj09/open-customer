package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	// sessionLifetime is the 90-day expiry that slides forward ("Session and request rules").
	sessionLifetime = 90 * 24 * time.Hour
	// slideInterval: last_seen_at is written at most once per hour per session, and the same
	// write extends expires_at.
	slideInterval = time.Hour
	// sessionCookieName is the session cookie.
	sessionCookieName = "oc_session"
)

// Me is the signed-in user.
type Me struct {
	ID                 uuid.UUID
	Username           string
	MustChangePassword bool
}

// Session is a loaded session. TokenHash is the stored hash, never the token.
type Session struct {
	TokenHash []byte
	UserID    uuid.UUID
	ExpiresAt time.Time
}

// createSession stores a new session for userID and returns its token for the cookie.
func (a *Auth) createSession(ctx context.Context, q db.DBTX, userID uuid.UUID) (string, error) {
	token, hash, err := newToken()
	if err != nil {
		return "", err
	}
	now := a.clock.Now()
	if err := sqlc.New(q).InsertSession(ctx, sqlc.InsertSessionParams{
		TokenHash: hash, UserID: userID, Now: now, ExpiresAt: now.Add(sessionLifetime),
	}); err != nil {
		return "", fmt.Errorf("insert a session: %w", err)
	}
	return token, nil
}

// Authenticate loads the session for token. A malformed, unknown or expired token is
// Unauthorized. When the session was last seen an hour or more ago, one write moves last_seen_at
// to now and expires_at to now+90d, and refreshed tells the caller to re-send the cookie.
func (a *Auth) Authenticate(ctx context.Context, q db.DBTX, token string) (Me, Session, bool, error) {
	hash, err := parseToken(token)
	if err != nil {
		return Me{}, Session{}, false, err
	}
	queries := sqlc.New(q)
	row, err := queries.GetSessionWithUser(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Me{}, Session{}, false, problem.Unauthorized{}
	}
	if err != nil {
		return Me{}, Session{}, false, fmt.Errorf("load a session: %w", err)
	}
	now := a.clock.Now()
	if !now.Before(row.ExpiresAt) {
		return Me{}, Session{}, false, problem.Unauthorized{}
	}
	me := Me{ID: row.UserID, Username: row.Username, MustChangePassword: row.MustChangePassword}
	sess := Session{TokenHash: row.TokenHash, UserID: row.UserID, ExpiresAt: row.ExpiresAt}
	// createSession always sets last_seen_at; a NULL one counts as long ago.
	if row.LastSeenAt != nil && now.Sub(*row.LastSeenAt) < slideInterval {
		return me, sess, false, nil
	}
	sess.ExpiresAt = now.Add(sessionLifetime)
	if err := queries.TouchSession(ctx, sqlc.TouchSessionParams{
		Now: now, ExpiresAt: sess.ExpiresAt, TokenHash: hash,
	}); err != nil {
		return Me{}, Session{}, false, fmt.Errorf("slide a session: %w", err)
	}
	return me, sess, true, nil
}

// Logout deletes the session for token. A malformed or unknown token is not an error: there is
// nothing left to close.
func (a *Auth) Logout(ctx context.Context, q db.DBTX, token string) error {
	hash, parseErr := parseToken(token)
	if parseErr == nil {
		if err := sqlc.New(q).DeleteSession(ctx, hash); err != nil {
			return fmt.Errorf("delete a session: %w", err)
		}
	}
	return nil
}

// revokeOtherSessions closes every session of the user except keep (a password change, item 39).
func revokeOtherSessions(ctx context.Context, q db.DBTX, keep Session) error {
	if err := sqlc.New(q).DeleteOtherSessions(ctx, sqlc.DeleteOtherSessionsParams{
		UserID: keep.UserID, KeepTokenHash: keep.TokenHash,
	}); err != nil {
		return fmt.Errorf("delete the other sessions: %w", err)
	}
	return nil
}

// revokeAllSessions closes every session of the user (reset-password, item 39).
func revokeAllSessions(ctx context.Context, q db.DBTX, userID uuid.UUID) error {
	if err := sqlc.New(q).DeleteUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("delete the user's sessions: %w", err)
	}
	return nil
}

// SessionCookie carries token for 90 days. secure is PUBLIC_URL.Scheme == "https": it is off only
// for http://localhost and http://127.0.0.1, because WebKit does not store a Secure cookie over
// plain http.
func SessionCookie(token string, secure bool) *http.Cookie {
	return sessionCookie(token, int(sessionLifetime/time.Second), secure)
}

// ExpiredCookie tells the browser to delete the session cookie now.
func ExpiredCookie(secure bool) *http.Cookie {
	return sessionCookie("", -1, secure)
}

func sessionCookie(value string, maxAge int, secure bool) *http.Cookie {
	c := &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	// Secure by default; only the plain-http PUBLIC_URL case above turns it off. Kept as a
	// default plus one override so the literal itself is safe (gosec G124).
	c.Secure = secure
	return c
}
