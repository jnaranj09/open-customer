//go:build db

package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var sessionsStart = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

// sessionFixture is a fresh database with one user, an Auth on a fake clock and one session.
type sessionFixture struct {
	pool   *pgxpool.Pool
	clock  *clock.Fake
	auth   *Auth
	userID uuid.UUID
	token  string
}

func newSessionFixture(t *testing.T) sessionFixture {
	t.Helper()
	pool := dbtest.Pool(t)
	fake := clock.NewFake(sessionsStart)
	a := NewForTest(t, Options{Clock: fake})
	userID := uuid.New()
	if err := sqlc.New(pool).InsertUser(t.Context(), sqlc.InsertUserParams{
		ID: userID, Username: "made-up-owner", PasswordHash: "x", CreatedAt: sessionsStart,
	}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	token, err := a.createSession(t.Context(), pool, userID)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	return sessionFixture{pool: pool, clock: fake, auth: a, userID: userID, token: token}
}

// row reads the stored session times for token directly from the table.
func (f sessionFixture) row(t *testing.T, token string) (expiresAt, lastSeenAt time.Time) {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	sum := sha256.Sum256(raw)
	if err := f.pool.QueryRow(t.Context(),
		`SELECT expires_at, last_seen_at FROM sessions WHERE token_hash = $1`, sum[:]).
		Scan(&expiresAt, &lastSeenAt); err != nil {
		t.Fatalf("read session row: %v", err)
	}
	return expiresAt, lastSeenAt
}

func wantUnauthorized(t *testing.T, err error) {
	t.Helper()
	var u problem.Unauthorized
	if !errors.As(err, &u) {
		t.Fatalf("err = %v; want problem.Unauthorized", err)
	}
}

func TestAuthenticateFreshSession(t *testing.T) {
	f := newSessionFixture(t)
	me, sess, refreshed, err := f.auth.Authenticate(t.Context(), f.pool, f.token)
	if err != nil || refreshed {
		t.Fatalf("Authenticate(new session) = refreshed %v, %v; want false, nil", refreshed, err)
	}
	if want := (Me{ID: f.userID, Username: "made-up-owner"}); me != want {
		t.Fatalf("Me = %+v; want %+v", me, want)
	}
	if sess.UserID != f.userID || !sess.ExpiresAt.Equal(sessionsStart.Add(90*24*time.Hour)) {
		t.Fatalf("Session = %+v; want user %v, expiry now+90d", sess, f.userID)
	}
}

func TestAuthenticateSlidesAfterAnHour(t *testing.T) {
	f := newSessionFixture(t)
	f.clock.Advance(61 * time.Minute)
	now := f.clock.Now()

	_, sess, refreshed, err := f.auth.Authenticate(t.Context(), f.pool, f.token)
	if err != nil || !refreshed {
		t.Fatalf("Authenticate(61 min later) = refreshed %v, %v; want true, nil", refreshed, err)
	}
	wantExpiry := now.Add(90 * 24 * time.Hour)
	if !sess.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("Session.ExpiresAt = %v; want %v", sess.ExpiresAt, wantExpiry)
	}
	expiresAt, lastSeenAt := f.row(t, f.token)
	if !expiresAt.Equal(wantExpiry) || !lastSeenAt.Equal(now) {
		t.Fatalf("stored expires_at %v, last_seen_at %v; want %v, %v", expiresAt, lastSeenAt, wantExpiry, now)
	}
}

func TestAuthenticateNoSlideWithinHour(t *testing.T) {
	f := newSessionFixture(t)
	f.clock.Advance(59 * time.Minute)

	_, sess, refreshed, err := f.auth.Authenticate(t.Context(), f.pool, f.token)
	if err != nil || refreshed {
		t.Fatalf("Authenticate(59 min later) = refreshed %v, %v; want false, nil", refreshed, err)
	}
	wantExpiry := sessionsStart.Add(90 * 24 * time.Hour)
	if !sess.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("Session.ExpiresAt = %v; want the original %v", sess.ExpiresAt, wantExpiry)
	}
	expiresAt, lastSeenAt := f.row(t, f.token)
	if !expiresAt.Equal(wantExpiry) || !lastSeenAt.Equal(sessionsStart) {
		t.Fatalf("stored expires_at %v, last_seen_at %v; want unchanged %v, %v",
			expiresAt, lastSeenAt, wantExpiry, sessionsStart)
	}
}

// logoutBeforeSlide is the real database, except that a concurrent logout deletes every session
// just before the sliding UPDATE runs (the window between Authenticate's SELECT and its UPDATE).
type logoutBeforeSlide struct {
	db.DBTX
}

func (l logoutBeforeSlide) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "-- name: TouchSession ") {
		if _, err := l.DBTX.Exec(ctx, `DELETE FROM sessions`); err != nil {
			return pgconn.CommandTag{}, err
		}
	}
	return l.DBTX.Exec(ctx, sql, args...)
}

func TestAuthenticateSessionDeletedBeforeSlide(t *testing.T) {
	f := newSessionFixture(t)
	f.clock.Advance(61 * time.Minute)
	_, _, refreshed, err := f.auth.Authenticate(t.Context(), logoutBeforeSlide{f.pool}, f.token)
	if refreshed {
		t.Fatalf("Authenticate(session deleted before the slide) refreshed = true; want false (no new cookie)")
	}
	wantUnauthorized(t, err)
}

func TestAuthenticateExpired(t *testing.T) {
	f := newSessionFixture(t)
	f.clock.Advance(91 * 24 * time.Hour)
	_, _, _, err := f.auth.Authenticate(t.Context(), f.pool, f.token)
	wantUnauthorized(t, err)
}

func TestAuthenticateBadToken(t *testing.T) {
	f := newSessionFixture(t)
	unknown, _, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	tests := []struct {
		name  string
		token string
	}{
		{"an empty token", ""},
		{"a short token", "abc"},
		{"a 44-character token", f.token + "A"},
		{"a token with a character outside base64url", "+" + f.token[1:]},
		{"a token with a padding character", f.token[:42] + "="},
		{"the same token with non-zero trailing bits (a second spelling)", flipTrailingBits(f.token)},
		{"the same token with a line feed inside (skipped by the decoder)", f.token[:20] + "\n" + f.token[20:]},
		{"a well-formed token with no session", unknown},
	}
	for _, tt := range tests {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			_, _, _, err := f.auth.Authenticate(t.Context(), f.pool, tt.token)
			wantUnauthorized(t, err)
		})
	}
}

func TestLogoutRevokes(t *testing.T) {
	f := newSessionFixture(t)
	if err := f.auth.Logout(t.Context(), f.pool, f.token); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	_, _, _, err := f.auth.Authenticate(t.Context(), f.pool, f.token)
	wantUnauthorized(t, err)
}

func TestLogoutUnknownTokenIsNotAnError(t *testing.T) {
	f := newSessionFixture(t)
	unknown, _, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	for _, token := range []string{unknown, "abc"} {
		if err := f.auth.Logout(t.Context(), f.pool, token); err != nil {
			t.Fatalf("Logout(token with no session) = %v; want nil", err)
		}
	}
	if _, _, _, err := f.auth.Authenticate(t.Context(), f.pool, f.token); err != nil {
		t.Fatalf("Authenticate(the other session) = %v; want nil (it must survive)", err)
	}
}

func TestTokenStoredAsHash(t *testing.T) {
	f := newSessionFixture(t)
	if len(f.token) != 43 {
		t.Fatalf("token length = %d; want 43 (32 bytes, unpadded base64url)", len(f.token))
	}
	raw, err := base64.RawURLEncoding.DecodeString(f.token)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token decodes to %d bytes, %v; want 32, nil", len(raw), err)
	}
	var stored []byte
	if err := f.pool.QueryRow(t.Context(), `SELECT token_hash FROM sessions WHERE user_id = $1`, f.userID).
		Scan(&stored); err != nil {
		t.Fatalf("read token_hash: %v", err)
	}
	sum := sha256.Sum256(raw)
	if bytes.Equal(stored, raw) || !bytes.Equal(stored, sum[:]) {
		t.Fatalf("token_hash = %x; want sha256 of the raw token %x, not the raw bytes", stored, sum)
	}
}

func TestRevokeOtherSessionsKeepsCurrent(t *testing.T) {
	f := newSessionFixture(t)
	other, err := f.auth.createSession(t.Context(), f.pool, f.userID)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	_, current, _, err := f.auth.Authenticate(t.Context(), f.pool, f.token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if err := revokeOtherSessions(t.Context(), f.pool, current); err != nil {
		t.Fatalf("revokeOtherSessions: %v", err)
	}
	if _, _, _, err := f.auth.Authenticate(t.Context(), f.pool, f.token); err != nil {
		t.Fatalf("Authenticate(kept session) = %v; want nil", err)
	}
	_, _, _, err = f.auth.Authenticate(t.Context(), f.pool, other)
	wantUnauthorized(t, err)
}

func TestRevokeAllSessions(t *testing.T) {
	f := newSessionFixture(t)
	other, err := f.auth.createSession(t.Context(), f.pool, f.userID)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if err := revokeAllSessions(t.Context(), f.pool, f.userID); err != nil {
		t.Fatalf("revokeAllSessions: %v", err)
	}
	for _, token := range []string{f.token, other} {
		_, _, _, err := f.auth.Authenticate(t.Context(), f.pool, token)
		wantUnauthorized(t, err)
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	t.Parallel()
	const token = "made-up-token"
	for _, tt := range []struct {
		name   string
		secure bool
	}{{"with Secure for an https PUBLIC_URL", true}, {"without Secure for http localhost", false}} {
		secure := tt.secure
		t.Run("session cookie "+tt.name, func(t *testing.T) {
			t.Parallel()
			c := SessionCookie(token, secure)
			if c.Name != "oc_session" || c.Value != token || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode ||
				c.Path != "/" || c.MaxAge != 90*24*60*60 || c.Secure != secure {
				t.Fatalf("SessionCookie(secure=%v) = %+v", secure, c)
			}
		})
		t.Run("expired cookie "+tt.name, func(t *testing.T) {
			t.Parallel()
			c := ExpiredCookie(secure)
			if c.Name != "oc_session" || c.Value != "" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode ||
				c.Path != "/" || c.MaxAge != -1 || c.Secure != secure {
				t.Fatalf("ExpiredCookie(secure=%v) = %+v", secure, c)
			}
			if h := c.String(); !strings.Contains(h, "Max-Age=0") {
				t.Fatalf("ExpiredCookie header = %q; want Max-Age=0 (delete now)", h)
			}
		})
	}
}
