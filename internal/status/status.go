// Package status answers the liveness and readiness probes (/healthz, /readyz). It owns no table.
package status

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
)

// Status holds what the probes report on. The version string is what main passes in; P4's status
// report will show it.
type Status struct {
	version string
}

// New returns a Status for the running build.
func New(version string) *Status {
	return &Status{version: version}
}

// Health reports that the process is alive. It touches no database and cannot fail.
func (s *Status) Health() bool {
	return true
}

// Ready reports whether the database answers and every embedded migration is applied. A failure
// logs a stable code, never the error text, and returns false.
func (s *Status) Ready(ctx context.Context, pool *pgxpool.Pool) bool {
	if err := pool.Ping(ctx); err != nil {
		slog.ErrorContext(ctx, "not ready", "code", "ready_db_unreachable")
		return false
	}
	pending, err := db.HasPending(ctx, pool)
	if err != nil {
		slog.ErrorContext(ctx, "not ready", "code", "ready_db_unreachable")
		return false
	}
	if pending {
		slog.ErrorContext(ctx, "not ready", "code", "ready_migrations_pending")
		return false
	}
	return true
}
