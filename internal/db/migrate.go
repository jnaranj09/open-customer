package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/jnaranj09/open-customer/internal/platform/backupstatus"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

//go:embed migrations/*.sql
var embedFS embed.FS

// maxBackupAge is how old last_copy_at may be before the guard blocks a migration.
const maxBackupAge = 60 * time.Minute

// noRecentBackupError is the guard's refusal. Its text is the operator-facing sentence the design
// doc names, so it starts with a capital letter and ends with a full stop.
type noRecentBackupError struct{}

func (noRecentBackupError) Error() string {
	return "Pending migrations and no recent backup. Run scripts/upgrade.sh."
}

// MigrateOptions configures Migrate.
type MigrateOptions struct {
	// AllowWithoutBackup skips the backup guard (ALLOW_MIGRATE_WITHOUT_BACKUP).
	AllowWithoutBackup bool
	// BackupStatusFile is the path of the backup container's status.json (BACKUP_STATUS_FILE).
	BackupStatusFile string
	// Clock supplies "now" for the age of the last copy. Required unless AllowWithoutBackup is set.
	Clock clock.Clock
	// Logger receives the guard's warnings; nil means slog.Default().
	Logger *slog.Logger
}

func (o MigrateOptions) logger() *slog.Logger {
	if o.Logger == nil {
		return slog.Default()
	}
	return o.Logger
}

// embeddedMigrations returns the embedded migrations at the root of the FS, which is where the
// goose Provider looks for them.
func embeddedMigrations() (fs.FS, error) {
	sub, err := fs.Sub(embedFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("db: embedded migrations: %w", err)
	}
	return sub, nil
}

// Migrate applies the pending migrations, unless the backup guard blocks them.
func Migrate(ctx context.Context, pool *pgxpool.Pool, opts MigrateOptions) error {
	fsys, err := embeddedMigrations()
	if err != nil {
		return err
	}
	return migrateFS(ctx, pool, fsys, opts)
}

// Version returns the goose version of the database: 0 when no migration was applied.
func Version(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	fsys, err := embeddedMigrations()
	if err != nil {
		return 0, err
	}
	return withProvider(pool, fsys, func(p *goose.Provider) (int64, error) {
		return p.GetDBVersion(ctx)
	})
}

// HasPending reports whether the embedded migrations include one the database has not applied.
func HasPending(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	fsys, err := embeddedMigrations()
	if err != nil {
		return false, err
	}
	return withProvider(pool, fsys, func(p *goose.Provider) (bool, error) {
		return p.HasPending(ctx)
	})
}

// migrateFS is Migrate over any migrations FS, so tests can give it a database that is
// already at version 1 with a second migration pending.
func migrateFS(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, opts MigrateOptions) error {
	_, err := withProvider(pool, fsys, func(p *goose.Provider) (struct{}, error) {
		if !opts.AllowWithoutBackup {
			if err := checkGuard(ctx, p, opts); err != nil {
				return struct{}{}, err
			}
		}
		if _, err := p.Up(ctx); err != nil {
			return struct{}{}, fmt.Errorf("db: migrate up: %w", err)
		}
		return struct{}{}, nil
	})
	return err
}

func checkGuard(ctx context.Context, p *goose.Provider, opts MigrateOptions) error {
	pending, err := p.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("db: check pending migrations: %w", err)
	}
	if !pending {
		return nil
	}
	version, err := p.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("db: read migration version: %w", err)
	}
	if version == 0 {
		return nil
	}
	if opts.Clock == nil {
		return errors.New("db: Migrate needs a Clock unless AllowWithoutBackup is set")
	}
	if guardBlocks(pending, version, lastCopy(ctx, opts), opts.Clock.Now()) {
		return noRecentBackupError{}
	}
	return nil
}

// lastCopy returns last_copy_at, or nil when the file is missing, unreadable or invalid, or the
// field is null. Every unreadable case fails closed (nil blocks) and logs a code, never the path.
func lastCopy(ctx context.Context, opts MigrateOptions) *time.Time {
	st, err := backupstatus.Read(opts.BackupStatusFile)
	switch {
	case err == nil:
		return st.LastCopyAt
	case errors.Is(err, backupstatus.ErrMissing):
		return nil
	case errors.Is(err, backupstatus.ErrInvalid):
		opts.logger().WarnContext(ctx, "backup status file invalid", "code", "backup_status_invalid")
		return nil
	default:
		opts.logger().WarnContext(ctx, "backup status file could not be read", "code", "backup_status_unreadable")
		return nil
	}
}

// guardBlocks is the migration guard (design doc, "Upgrades and the migration guard"): it blocks
// only when a migration is pending, the database is not fresh, and the last backup copy is
// missing or more than maxBackupAge old.
func guardBlocks(pending bool, version int64, lastCopyAt *time.Time, now time.Time) bool {
	if !pending || version <= 0 {
		return false
	}
	return lastCopyAt == nil || now.Sub(*lastCopyAt) > maxBackupAge
}

// withProvider builds a goose Provider over a *sql.DB borrowed from the pool and closes that
// *sql.DB, never the pool.
func withProvider[T any](pool *pgxpool.Pool, fsys fs.FS, fn func(*goose.Provider) (T, error)) (T, error) {
	var zero T
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer closeSQL(sqlDB)
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys)
	if err != nil {
		return zero, fmt.Errorf("db: goose provider: %w", err)
	}
	return fn(p)
}

func closeSQL(db *sql.DB) {
	if err := db.Close(); err != nil {
		slog.Warn("closing migration connection failed", "code", "migrate_close_failed")
	}
}
