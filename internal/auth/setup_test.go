//go:build db

package auth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var setupStart = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

func TestSetupNeeded(t *testing.T) {
	pool := dbtest.Pool(t)
	a := auth.NewForTest(t, auth.Options{Clock: clock.NewFake(setupStart)})

	needed, err := a.SetupNeeded(t.Context(), pool)
	if err != nil || !needed {
		t.Fatalf("empty database: SetupNeeded = %v, %v; want true, nil", needed, err)
	}

	insertUser(t, pool)
	needed, err = a.SetupNeeded(t.Context(), pool)
	if err != nil || needed {
		t.Fatalf("after a user insert: SetupNeeded = %v, %v; want false, nil", needed, err)
	}
}

// setupFixture is a fresh database, an Auth on a fake clock whose settings creator records its
// input, and one setup token.
type setupFixture struct {
	pool     *pgxpool.Pool
	clock    *clock.Fake
	auth     *auth.Auth
	token    string
	created  []auth.SetupSettings
	createFn func(ctx context.Context, q db.DBTX, s auth.SetupSettings) error
}

func newSetupFixture(t *testing.T) *setupFixture {
	t.Helper()
	f := &setupFixture{pool: dbtest.Pool(t), clock: clock.NewFake(setupStart)}
	f.auth = auth.NewForTest(t, auth.Options{
		Clock: f.clock,
		SettingsCreator: func(ctx context.Context, q db.DBTX, s auth.SetupSettings) error {
			f.created = append(f.created, s)
			if f.createFn != nil {
				return f.createFn(ctx, q, s)
			}
			return nil
		},
	})
	token, err := f.auth.NewSetupToken(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("NewSetupToken: %v", err)
	}
	f.token = token
	return f
}

func validSetupInput(token string) auth.SetupInput {
	hour := 9
	return auth.SetupInput{
		Token: token, Username: " Ana ", Password: "made-up-password",
		Settings: auth.SetupSettings{
			Name: "Made-up Studio", Timezone: "America/Bogota", Locale: "es", Currency: "COP",
			CountryCode: "CO", AccentColor: "#aa3366", PushHour: &hour,
		},
	}
}

func insertUser(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if err := sqlc.New(pool).InsertUser(t.Context(), sqlc.InsertUserParams{
		ID: uuid.New(), Username: "made-up-owner", PasswordHash: "x", CreatedAt: setupStart,
	}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func count(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func wantSetupTokenInvalid(t *testing.T, err error) {
	t.Helper()
	var p problem.SetupTokenInvalid
	if !errors.As(err, &p) {
		t.Fatalf("err = %v; want problem.SetupTokenInvalid", err)
	}
}

func wantSetupClosed(t *testing.T, err error) {
	t.Helper()
	var p problem.SetupClosed
	if !errors.As(err, &p) {
		t.Fatalf("err = %v; want problem.SetupClosed", err)
	}
}

func TestNewSetupTokenReplacesOld(t *testing.T) {
	f := newSetupFixture(t)
	second, err := f.auth.NewSetupToken(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("second NewSetupToken: %v", err)
	}
	if n := count(t, f.pool, "setup_tokens"); n != 1 {
		t.Fatalf("setup_tokens rows = %d, want 1", n)
	}
	_, _, err = f.auth.CompleteSetup(t.Context(), f.pool, validSetupInput(f.token))
	wantSetupTokenInvalid(t, err)
	if _, _, err := f.auth.CompleteSetup(t.Context(), f.pool, validSetupInput(second)); err != nil {
		t.Fatalf("CompleteSetup with the new token: %v", err)
	}
}

func TestSetupTokenStoredAsHash(t *testing.T) {
	f := newSetupFixture(t)
	raw, err := base64.RawURLEncoding.DecodeString(f.token)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token decodes to %d bytes, %v; want 32, nil", len(raw), err)
	}
	var stored []byte
	if err := f.pool.QueryRow(t.Context(), `SELECT token_hash FROM setup_tokens`).Scan(&stored); err != nil {
		t.Fatalf("read token_hash: %v", err)
	}
	sum := sha256.Sum256(raw)
	if bytes.Equal(stored, raw) || !bytes.Equal(stored, sum[:]) {
		t.Fatalf("token_hash = %x; want sha256 of the raw token %x, not the raw bytes", stored, sum)
	}
}

func TestNewSetupTokenRefusedWhenUserExists(t *testing.T) {
	f := newSetupFixture(t)
	insertUser(t, f.pool)
	token, err := f.auth.NewSetupToken(t.Context(), f.pool)
	wantSetupClosed(t, err)
	if token != "" || count(t, f.pool, "setup_tokens") != 1 {
		t.Fatalf("token = %q, setup_tokens rows = %d; want no token and the old row untouched",
			token, count(t, f.pool, "setup_tokens"))
	}
}

func TestCompleteSetupHappyPath(t *testing.T) {
	f := newSetupFixture(t)
	in := validSetupInput(f.token)
	me, token, err := f.auth.CompleteSetup(t.Context(), f.pool, in)
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}
	if me.Username != "ana" || me.MustChangePassword || me.ID == uuid.Nil {
		t.Fatalf("Me = %+v; want username ana, must_change_password false and an id", me)
	}
	got, _, _, err := f.auth.Authenticate(t.Context(), f.pool, token)
	if err != nil || got != me {
		t.Fatalf("Authenticate(session token) = %+v, %v; want %+v, nil", got, err, me)
	}
	if !reflect.DeepEqual(f.created, []auth.SetupSettings{in.Settings}) {
		t.Fatalf("settings creator got %+v; want one call with %+v", f.created, in.Settings)
	}
	if n := count(t, f.pool, "setup_tokens"); n != 0 {
		t.Fatalf("setup_tokens rows = %d, want 0", n)
	}
	var mustChange bool
	if err := f.pool.QueryRow(t.Context(), `SELECT must_change_password FROM users WHERE username = 'ana'`).
		Scan(&mustChange); err != nil || mustChange {
		t.Fatalf("users row: must_change_password = %v, %v; want false, nil", mustChange, err)
	}
}

func TestSetupTokenCannotBeReused(t *testing.T) {
	f := newSetupFixture(t)
	if _, _, err := f.auth.CompleteSetup(t.Context(), f.pool, validSetupInput(f.token)); err != nil {
		t.Fatalf("first CompleteSetup: %v", err)
	}
	_, _, err := f.auth.CompleteSetup(t.Context(), f.pool, validSetupInput(f.token))
	wantSetupClosed(t, err)
}

func TestSetupRefusedOnceUserExists(t *testing.T) {
	f := newSetupFixture(t)
	insertUser(t, f.pool)
	_, _, err := f.auth.CompleteSetup(t.Context(), f.pool, validSetupInput(f.token))
	wantSetupClosed(t, err)
	if len(f.created) != 0 || count(t, f.pool, "users") != 1 {
		t.Fatalf("creator calls = %d, users = %d; want 0 and the one existing user",
			len(f.created), count(t, f.pool, "users"))
	}
}

func TestSetupTokenExpired(t *testing.T) {
	f := newSetupFixture(t)
	f.clock.Advance(24*time.Hour + time.Second)
	_, _, err := f.auth.CompleteSetup(t.Context(), f.pool, validSetupInput(f.token))
	wantSetupTokenInvalid(t, err)
}

func TestSetupTokenGarbage(t *testing.T) {
	cases := []struct{ name, token string }{
		{"an empty token", ""},
		{"a one-character token", "x"},
		{"a well-formed token that was never issued", strings.Repeat("A", 43)},
	}
	for _, tt := range cases {
		t.Run(tt.name+" is setup-token-invalid", func(t *testing.T) {
			f := newSetupFixture(t)
			_, _, err := f.auth.CompleteSetup(t.Context(), f.pool, validSetupInput(tt.token))
			wantSetupTokenInvalid(t, err)
		})
	}
}

func TestSetupBadUsername(t *testing.T) {
	f := newSetupFixture(t)
	in := validSetupInput(f.token)
	in.Username = "a"
	_, _, err := f.auth.CompleteSetup(t.Context(), f.pool, in)
	var v problem.Validation
	if !errors.As(err, &v) || len(v.Errors) != 1 || v.Errors[0].Field != "username" {
		t.Fatalf("err = %v; want a Validation on username", err)
	}
	if len(f.created) != 0 || count(t, f.pool, "users") != 0 {
		t.Fatalf("creator calls = %d, users = %d; want 0 and 0", len(f.created), count(t, f.pool, "users"))
	}
}

func TestSetupShortPassword(t *testing.T) {
	f := newSetupFixture(t)
	in := validSetupInput(f.token)
	in.Password = "1234567"
	_, _, err := f.auth.CompleteSetup(t.Context(), f.pool, in)
	var v problem.Validation
	if !errors.As(err, &v) || len(v.Errors) != 1 || v.Errors[0].Field != "password" {
		t.Fatalf("err = %v; want a Validation on password", err)
	}
	if len(f.created) != 0 || count(t, f.pool, "users") != 0 {
		t.Fatalf("creator calls = %d, users = %d; want 0 and 0", len(f.created), count(t, f.pool, "users"))
	}
}

func TestSetupBadSettingsRollsBackInTx(t *testing.T) {
	f := newSetupFixture(t)
	// The creator writes in the transaction before it fails, as a partial insert would.
	f.createFn = func(ctx context.Context, q db.DBTX, _ auth.SetupSettings) error {
		if _, err := q.Exec(ctx, `INSERT INTO users (id, username, password_hash) VALUES ($1, 'stray', 'x')`,
			uuid.New()); err != nil {
			return err
		}
		return problem.Validation{Errors: []problem.FieldError{{Field: "timezone", Message: problem.MsgTimezoneInvalid}}}
	}
	err := db.WithTx(t.Context(), f.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, _, err := f.auth.CompleteSetup(t.Context(), tx, validSetupInput(f.token))
		return err
	})
	var v problem.Validation
	if !errors.As(err, &v) || len(v.Errors) != 1 || v.Errors[0].Field != "settings.timezone" ||
		v.Errors[0].Message != problem.MsgTimezoneInvalid {
		t.Fatalf("err = %v; want a Validation on settings.timezone", err)
	}
	if n := count(t, f.pool, "users"); n != 0 {
		t.Fatalf("users rows after the rollback = %d, want 0", n)
	}
}

// begin opens a transaction the test commits itself; cleanup rolls back whatever is left open.
func begin(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.WithoutCancel(t.Context())); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback: %v", err)
		}
	})
	return tx
}

// waitBlockedOrDone returns once done is closed or a backend of this test's database waits on a
// lock. It polls pg_stat_activity, so the test needs no sleep.
func waitBlockedOrDone(t *testing.T, pool *pgxpool.Pool, done <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	for {
		select {
		case <-done:
			return
		default:
		}
		var waiting int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatalf("poll pg_stat_activity: %v", err)
		}
		if waiting > 0 {
			return
		}
	}
}

func TestConcurrentSetupWithOneTokenCreatesOneUser(t *testing.T) {
	// The fake creator writes nothing, so the settings primary key cannot be what stops the second
	// setup: only the setup lock can.
	f := newSetupFixture(t)
	tx1, tx2 := begin(t, f.pool), begin(t, f.pool)
	if _, _, err := f.auth.CompleteSetup(t.Context(), tx1, validSetupInput(f.token)); err != nil {
		t.Fatalf("first CompleteSetup: %v", err)
	}
	second := validSetupInput(f.token)
	second.Username = "bea"
	var secondErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, secondErr = f.auth.CompleteSetup(t.Context(), tx2, second)
	}()
	waitBlockedOrDone(t, f.pool, done)
	if err := tx1.Commit(t.Context()); err != nil {
		t.Fatalf("commit the first setup: %v", err)
	}
	<-done
	wantSetupClosed(t, secondErr)
	if n := count(t, f.pool, "users"); n != 1 {
		t.Fatalf("users rows = %d, want 1", n)
	}
}

func TestConcurrentNewSetupTokenLeavesOneToken(t *testing.T) {
	f := newSetupFixture(t)
	tx1, tx2 := begin(t, f.pool), begin(t, f.pool)
	if _, err := f.auth.NewSetupToken(t.Context(), tx1); err != nil {
		t.Fatalf("first NewSetupToken: %v", err)
	}
	var secondErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, secondErr = f.auth.NewSetupToken(t.Context(), tx2)
	}()
	waitBlockedOrDone(t, f.pool, done)
	if err := tx1.Commit(t.Context()); err != nil {
		t.Fatalf("commit the first token: %v", err)
	}
	<-done
	if secondErr != nil {
		t.Fatalf("second NewSetupToken: %v", secondErr)
	}
	if err := tx2.Commit(t.Context()); err != nil {
		t.Fatalf("commit the second token: %v", err)
	}
	if n := count(t, f.pool, "setup_tokens"); n != 1 {
		t.Fatalf("setup_tokens rows = %d, want 1", n)
	}
}
