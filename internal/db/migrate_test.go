//go:build db

package db_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

const guardMessage = "Pending migrations and no recent backup. Run scripts/upgrade.sh."

var (
	testNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	migrationOne = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE first_table (id int);\n")}
	migrationTwo = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE second_table (id int);\n")}
)

func oneMigration() fs.FS {
	return fstest.MapFS{"00001_first.sql": migrationOne}
}

func twoMigrations() fs.FS {
	return fstest.MapFS{"00001_first.sql": migrationOne, "00002_second.sql": migrationTwo}
}

func emptyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := db.Open(t.Context(), dbtest.Empty(t).URL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// poolAtVersionOne returns a database where only the first of twoMigrations() has been applied.
func poolAtVersionOne(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := emptyPool(t)
	if err := db.MigrateFS(t.Context(), pool, oneMigration(), db.MigrateOptions{AllowWithoutBackup: true}); err != nil {
		t.Fatalf("apply first migration: %v", err)
	}
	return pool
}

func writeStatus(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write status file: %v", err)
	}
	return path
}

func statusWithCopy(t *testing.T, ago time.Duration) string {
	t.Helper()
	return writeStatus(t, `{"last_copy_at":"`+testNow.Add(-ago).Format("2006-01-02T15:04:05Z")+`"}`)
}

func optionsAt(path string) db.MigrateOptions {
	return db.MigrateOptions{BackupStatusFile: path, Clock: clock.NewFake(testNow)}
}

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var ok bool
	err := pool.QueryRow(t.Context(), `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&ok)
	if err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return ok
}

func TestEmbeddedFSHasMigrations(t *testing.T) {
	t.Parallel()
	fsys, err := db.EmbeddedMigrations()
	if err != nil {
		t.Fatalf("EmbeddedMigrations: %v", err)
	}
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("embedded FS has no .sql file at its root; check the fs.Sub path")
	}
}

func TestMigrateFreshDatabase(t *testing.T) {
	t.Parallel()
	pool := emptyPool(t)
	opts := optionsAt(filepath.Join(t.TempDir(), "absent.json"))
	if err := db.Migrate(t.Context(), pool, opts); err != nil {
		t.Fatalf("Migrate on a fresh database with no status file: %v", err)
	}
	v, err := db.Version(t.Context(), pool)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v < 1 {
		t.Fatalf("Version = %d, want at least 1", v)
	}
}

func TestMigrateBlocksWithOldCopy(t *testing.T) {
	t.Parallel()
	pool := poolAtVersionOne(t)
	err := db.MigrateFS(t.Context(), pool, twoMigrations(), optionsAt(statusWithCopy(t, 2*time.Hour)))
	if err == nil || err.Error() != guardMessage {
		t.Fatalf("MigrateFS error = %v, want %q", err, guardMessage)
	}
	if tableExists(t, pool, "second_table") {
		t.Fatal("the second migration was applied despite the guard")
	}
}

func TestMigrateAllowsRecentCopy(t *testing.T) {
	t.Parallel()
	pool := poolAtVersionOne(t)
	if err := db.MigrateFS(t.Context(), pool, twoMigrations(), optionsAt(statusWithCopy(t, 30*time.Minute))); err != nil {
		t.Fatalf("MigrateFS with a 30-minute-old copy: %v", err)
	}
	if !tableExists(t, pool, "second_table") {
		t.Fatal("the second migration was not applied")
	}
}

func TestMigrateBlocksOnMalformedStatusFile(t *testing.T) {
	t.Parallel()
	pool := poolAtVersionOne(t)
	err := db.MigrateFS(t.Context(), pool, twoMigrations(), optionsAt(writeStatus(t, "{not json")))
	if err == nil || err.Error() != guardMessage {
		t.Fatalf("MigrateFS error = %v, want %q", err, guardMessage)
	}
	if tableExists(t, pool, "second_table") {
		t.Fatal("the second migration was applied despite the guard")
	}
}

func TestMigrateAllowWithoutBackupSkipsGuard(t *testing.T) {
	t.Parallel()
	pool := poolAtVersionOne(t)
	opts := optionsAt(filepath.Join(t.TempDir(), "absent.json"))
	opts.AllowWithoutBackup = true
	if err := db.MigrateFS(t.Context(), pool, twoMigrations(), opts); err != nil {
		t.Fatalf("MigrateFS with AllowWithoutBackup: %v", err)
	}
	if !tableExists(t, pool, "second_table") {
		t.Fatal("the second migration was not applied")
	}
}

func TestMigrateNoPendingNeedsNoBackup(t *testing.T) {
	t.Parallel()
	pool := poolAtVersionOne(t)
	opts := optionsAt(filepath.Join(t.TempDir(), "absent.json"))
	if err := db.MigrateFS(t.Context(), pool, oneMigration(), opts); err != nil {
		t.Fatalf("MigrateFS with nothing pending and no status file: %v", err)
	}
}

func TestHasPendingAndVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := emptyPool(t)

	pending, err := db.HasPending(ctx, pool)
	if err != nil {
		t.Fatalf("HasPending before migrating: %v", err)
	}
	if !pending {
		t.Fatal("HasPending = false on an unmigrated database, want true")
	}
	v, err := db.Version(ctx, pool)
	if err != nil {
		t.Fatalf("Version before migrating: %v", err)
	}
	if v != 0 {
		t.Fatalf("Version = %d on an unmigrated database, want 0", v)
	}

	if err := db.Migrate(ctx, pool, db.MigrateOptions{AllowWithoutBackup: true}); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	pending, err = db.HasPending(ctx, pool)
	if err != nil {
		t.Fatalf("HasPending after migrating: %v", err)
	}
	if pending {
		t.Fatal("HasPending = true after migrating, want false")
	}
	v, err = db.Version(ctx, pool)
	if err != nil {
		t.Fatalf("Version after migrating: %v", err)
	}
	if v < 1 {
		t.Fatalf("Version = %d after migrating, want at least 1", v)
	}
}

func TestDBTestPoolIsMigratedAndIsolated(t *testing.T) {
	t.Parallel()
	a, b := dbtest.Pool(t), dbtest.Pool(t)
	if !tableExists(t, a, "users") {
		t.Fatal("dbtest.Pool database has no users table")
	}
	if _, err := a.Exec(t.Context(), `INSERT INTO business_settings (id, name, timezone, locale, currency, country_code, accent_color)
		VALUES (1, 'n', 'UTC', 'xx', 'XXX', 'XX', '#000000')`); err != nil {
		t.Fatalf("insert into first database: %v", err)
	}
	var n int
	if err := b.QueryRow(t.Context(), `SELECT count(*) FROM business_settings`).Scan(&n); err != nil {
		t.Fatalf("count in second database: %v", err)
	}
	if n != 0 {
		t.Fatalf("second database has %d business_settings rows, want 0", n)
	}
}
