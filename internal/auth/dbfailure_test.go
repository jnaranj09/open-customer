//go:build db

package auth

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

// errInjected is the failure failingDB returns from its chosen call.
var errInjected = errors.New("made-up database failure")

// failingDB is the real database, except that call number failAt (1-based; 0 never fails) returns
// errInjected instead of running. calls counts every call, so a clean run tells how many steps an
// operation has.
type failingDB struct {
	db.DBTX
	failAt int
	calls  *int
}

func (f failingDB) fail() bool {
	*f.calls++
	return *f.calls == f.failAt
}

func (f failingDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.fail() {
		return pgconn.CommandTag{}, errInjected
	}
	return f.DBTX.Exec(ctx, sql, args...)
}

func (f failingDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.fail() {
		return nil, errInjected
	}
	return f.DBTX.Query(ctx, sql, args...)
}

func (f failingDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.fail() {
		return failedRow{}
	}
	return f.DBTX.QueryRow(ctx, sql, args...)
}

type failedRow struct{}

func (failedRow) Scan(...any) error { return errInjected }

// dbOperation builds a fresh fixture on the real database and returns its pool and the one call
// under test.
type dbOperation struct {
	name  string
	build func(t *testing.T) (db.DBTX, func(q db.DBTX) error)
}

// newEmptyAuth is a fresh database with no user (setup still open) and an Auth on a fake clock.
func newEmptyAuth(t *testing.T) (*pgxpool.Pool, *Auth) {
	t.Helper()
	return dbtest.Pool(t), NewForTest(t, Options{Clock: clock.NewFake(sessionsStart)})
}

var dbOperations = []dbOperation{
	{"SetupNeeded", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		pool, a := newEmptyAuth(t)
		return pool, func(q db.DBTX) error { _, err := a.SetupNeeded(t.Context(), q); return err }
	}},
	{"NewSetupToken", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		pool, a := newEmptyAuth(t)
		return pool, func(q db.DBTX) error { _, err := a.NewSetupToken(t.Context(), q); return err }
	}},
	{"CompleteSetup", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		pool, a := newEmptyAuth(t)
		token, err := a.NewSetupToken(t.Context(), pool)
		if err != nil {
			t.Fatalf("NewSetupToken: %v", err)
		}
		in := SetupInput{Token: token, Username: "made-up-owner", Password: madeUpPassword}
		return pool, func(q db.DBTX) error { _, _, err := a.CompleteSetup(t.Context(), q, in); return err }
	}},
	{"Login", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newPasswordFixture(t)
		in := LoginInput{Username: "made-up-owner", Password: madeUpPassword}
		ip := netip.MustParseAddr("192.0.2.30")
		return f.pool, func(q db.DBTX) error { _, _, err := f.auth.Login(t.Context(), q, in, ip); return err }
	}},
	{"Authenticate with a slide", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newSessionFixture(t)
		f.clock.Advance(61 * time.Minute)
		return f.pool, func(q db.DBTX) error { _, _, _, err := f.auth.Authenticate(t.Context(), q, f.token); return err }
	}},
	{"Logout", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newSessionFixture(t)
		return f.pool, func(q db.DBTX) error { return f.auth.Logout(t.Context(), q, f.token) }
	}},
	{"ChangePassword", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newPasswordFixture(t)
		return f.pool, func(q db.DBTX) error {
			return f.auth.ChangePassword(t.Context(), q, f.session, madeUpPassword, madeUpNewPassword)
		}
	}},
	{"ResetPassword", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newPasswordFixture(t)
		return f.pool, func(q db.DBTX) error { _, err := f.auth.ResetPassword(t.Context(), q, "made-up-owner"); return err }
	}},
}

// A database failure at any step reaches the caller wrapped, so the handler's transaction rolls
// back and the request is a 500: never a success, and never a problem such as Unauthorized (a
// sign-out), InvalidCredentials or NotFound that would hide the outage.
func TestEveryDatabaseFailureReachesTheCaller(t *testing.T) {
	for _, op := range dbOperations {
		t.Run(op.name, func(t *testing.T) {
			steps := 0
			pool, run := op.build(t)
			if err := run(failingDB{DBTX: pool, calls: &steps}); err != nil {
				t.Fatalf("clean run: %v", err)
			}
			if steps == 0 {
				t.Fatal("clean run made no database call")
			}
			for step := 1; step <= steps; step++ {
				calls := 0
				pool, run := op.build(t)
				err := run(failingDB{DBTX: pool, failAt: step, calls: &calls})
				if !errors.Is(err, errInjected) {
					t.Fatalf("failure at step %d of %d: err = %v; want the database error wrapped", step, steps, err)
				}
			}
		})
	}
}
