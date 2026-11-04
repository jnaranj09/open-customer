//go:build db

package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
)

const madeUpOwner = "made-up-owner"

// openPool opens a pool on c that the test closes.
func openPool(t *testing.T, c dbtest.Conn) *pgxpool.Pool {
	t.Helper()
	pool, err := db.Open(t.Context(), c.URL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// withOwner returns a migrated database that already has the owner user madeUpOwner.
func withOwner(t *testing.T) dbtest.Conn {
	t.Helper()
	c := dbtest.Migrated(t)
	if err := sqlc.New(openPool(t, c)).InsertUser(t.Context(), sqlc.InsertUserParams{
		ID: uuid.New(), Username: madeUpOwner, PasswordHash: "x", CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return c
}

// runAdmin runs one subcommand against the database c and returns its exit code and output.
func runAdmin(t *testing.T, c dbtest.Conn, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(t.Context(), args, connEnv(t, c), &out, &errOut)
	return code, out.String(), errOut.String()
}

// wantFailure checks a failed subcommand: exit 1, nothing on stdout, msg on stderr.
func wantFailure(t *testing.T, code int, stdout, stderr, msg string) {
	t.Helper()
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want it empty", stdout)
	}
	if !strings.Contains(stderr, msg) {
		t.Errorf("stderr = %q, want it to contain %q", stderr, msg)
	}
}

func TestResetPasswordCommand(t *testing.T) {
	t.Parallel()
	c := withOwner(t)

	code, stdout, stderr := runAdmin(t, c, "reset-password", madeUpOwner)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr %q)", code, stderr)
	}
	if !regexp.MustCompile(`^[a-zA-Z2-9]{12}\n$`).MatchString(stdout) {
		t.Errorf("stdout = %q, want one 12-character temporary password", stdout)
	}
	var mustChange bool
	if err := openPool(t, c).QueryRow(t.Context(),
		"SELECT must_change_password FROM users WHERE username = $1", madeUpOwner).Scan(&mustChange); err != nil {
		t.Fatalf("read must_change_password: %v", err)
	}
	if !mustChange {
		t.Error("must_change_password = false after reset-password, want true")
	}
}

func TestResetPasswordUnknownUser(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runAdmin(t, withOwner(t), "reset-password", "someone-else")
	wantFailure(t, code, stdout, stderr, "no such user")
}

func TestResetPasswordMalformedUsername(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runAdmin(t, withOwner(t), "reset-password", "a")
	wantFailure(t, code, stdout, stderr, "invalid username")
}

func TestSetupLinkWhenNoUser(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runAdmin(t, dbtest.Migrated(t), "setup-link")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr %q)", code, stderr)
	}
	if !regexp.MustCompile(`^http://localhost:5173/setup#[A-Za-z0-9_-]{43}\n$`).MatchString(stdout) {
		t.Errorf("stdout = %q, want the setup link with a 43-character base64url token", stdout)
	}
}

func TestSetupLinkWhenUserExists(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runAdmin(t, withOwner(t), "setup-link")
	wantFailure(t, code, stdout, stderr, "setup is already complete")
}

func TestAdminCommandsRefusePendingMigrations(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"reset-password", madeUpOwner}, {"setup-link"}} {
		t.Run(args[0]+" on an unmigrated database exits 1", func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := runAdmin(t, dbtest.Empty(t), args...)
			wantFailure(t, code, stdout, stderr, "migrations pending; start serve first")
		})
	}
}
