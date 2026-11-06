//go:build db

package auth

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

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
		attempt, err := f.auth.BeginLogin(in, netip.MustParseAddr("192.0.2.30"))
		if err != nil {
			t.Fatalf("BeginLogin: %v", err)
		}
		return f.pool, func(q db.DBTX) error { _, _, err := f.auth.Login(t.Context(), q, attempt, in.Password); return err }
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
		session := f.authenticated(t, f.token)
		return f.pool, func(q db.DBTX) error { return f.auth.Logout(t.Context(), q, session) }
	}},
	{"ChangePassword", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newPasswordFixture(t)
		attempt, err := f.auth.BeginPasswordChange(f.me)
		if err != nil {
			t.Fatalf("BeginPasswordChange: %v", err)
		}
		return f.pool, func(q db.DBTX) error {
			return f.auth.ChangePassword(t.Context(), q, f.session, attempt, madeUpPassword, madeUpNewPassword)
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
			if err := run(dbtest.FailingDB{DBTX: pool, Calls: &steps}); err != nil {
				t.Fatalf("clean run: %v", err)
			}
			if steps == 0 {
				t.Fatal("clean run made no database call")
			}
			for step := 1; step <= steps; step++ {
				calls := 0
				pool, run := op.build(t)
				err := run(dbtest.FailingDB{DBTX: pool, FailAt: step, Calls: &calls})
				if !errors.Is(err, dbtest.ErrInjected) {
					t.Fatalf("failure at step %d of %d: err = %v; want the database error wrapped", step, steps, err)
				}
			}
		})
	}
}
