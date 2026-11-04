// Package status answers the liveness and readiness probes (/healthz, /readyz), builds the status
// report (GET /status) and logs the PWA's client errors (POST /client-errors). It owns no table
// and calls only read functions of other modules.
package status

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sys/unix"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/platform/backupstatus"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/push"
	"github.com/jnaranj09/open-customer/internal/scheduler"
)

// The backup line is red when the last copy is older than backupCopyMaxAge or the backup
// container has not checked in for backupCheckMaxAge (design doc "Backups").
const (
	backupCopyMaxAge  = 48 * time.Hour
	backupCheckMaxAge = 2 * time.Hour
)

// Options is what serve passes to New.
type Options struct {
	Version          string
	Clock            clock.Clock
	MediaDir         string
	BackupStatusFile string
	Push             *push.Module
	Scheduler        *scheduler.Scheduler
}

// Status holds what the probes, the report and the client-error log need.
type Status struct {
	version    string
	clock      clock.Clock
	mediaDir   string
	backupFile string
	push       *push.Module
	scheduler  *scheduler.Scheduler
	limiter    *clientErrorLimiter
}

// New returns a Status for the running build.
func New(opts Options) *Status {
	return &Status{
		version:    opts.Version,
		clock:      opts.Clock,
		mediaDir:   opts.MediaDir,
		backupFile: opts.BackupStatusFile,
		push:       opts.Push,
		scheduler:  opts.Scheduler,
		limiter:    newClientErrorLimiter(),
	}
}

// Backup is the backup line of the report. A nil time was null in the file, or the file is
// missing or unreadable (then every time is nil and Red is true).
type Backup struct {
	CheckedAt, LastDumpAt, LastCopyAt *time.Time
	LastError                         *string
	Red                               bool
}

// PushReport is the push line of the report.
type PushReport struct {
	Subscriptions int
	LastPush      *scheduler.LastPush
	Red           bool
}

// Report is the status screen data. A nil HeartbeatAgeS means the timer has not ticked yet; a nil
// MediaFreeBytes means the volume could not be read.
type Report struct {
	Version          string
	MigrationVersion int64
	HeartbeatAgeS    *int64
	Backup           Backup
	MediaFreeBytes   *int64
	Push             PushReport
}

// Status builds the report. A database error is returned wrapped; a missing backup file or media
// directory is part of the report, not an error.
func (s *Status) Status(ctx context.Context, pool *pgxpool.Pool) (Report, error) {
	version, err := db.Version(ctx, pool)
	if err != nil {
		return Report{}, fmt.Errorf("status: migration version: %w", err)
	}
	state, err := s.scheduler.State(ctx, pool)
	if err != nil {
		return Report{}, fmt.Errorf("status: scheduler state: %w", err)
	}
	subs, err := s.push.CountSubscriptions(ctx, pool)
	if err != nil {
		return Report{}, fmt.Errorf("status: count subscriptions: %w", err)
	}
	r := Report{
		Version:          s.version,
		MigrationVersion: version,
		Backup:           s.Backup(),
		MediaFreeBytes:   s.mediaFreeBytes(ctx),
		Push: PushReport{
			Subscriptions: subs,
			LastPush:      state.LastPush,
			Red:           pushRed(subs, state.LastPush),
		},
	}
	if state.Heartbeat != nil {
		age := max(int64(s.clock.Now().Sub(*state.Heartbeat)/time.Second), 0)
		r.HeartbeatAgeS = &age
	}
	return r, nil
}

// pushRed: nobody can be notified, or the last daily push reached nobody.
func pushRed(subscriptions int, last *scheduler.LastPush) bool {
	return subscriptions == 0 || (last != nil && last.Attempted > 0 && last.Delivered == 0)
}

// Backup reads BACKUP_STATUS_FILE and applies the red rules. A missing or unreadable file gives
// all times nil and Red true (the screen shows "desconocido").
func (s *Status) Backup() Backup {
	st, err := backupstatus.Read(s.backupFile)
	if err != nil {
		slog.Warn("backup status unavailable", "code", "backup_status_unavailable")
		return Backup{Red: true}
	}
	now := s.clock.Now()
	return Backup{
		CheckedAt:  st.CheckedAt,
		LastDumpAt: st.LastDumpAt,
		LastCopyAt: st.LastCopyAt,
		LastError:  st.LastError,
		Red: older(st.LastCopyAt, now, backupCopyMaxAge) || st.LastError != nil ||
			older(st.CheckedAt, now, backupCheckMaxAge),
	}
}

// BackupRed is Backup().Red, for the line on "Hoy" (/bootstrap).
func (s *Status) BackupRed() bool {
	return s.Backup().Red
}

// older reports whether t is missing or more than limit before now.
func older(t *time.Time, now time.Time, limit time.Duration) bool {
	return t == nil || now.Sub(*t) > limit
}

// mediaFreeBytes is the space an unprivileged process can still use on the media volume, or nil
// when it cannot be read.
func (s *Status) mediaFreeBytes(ctx context.Context) *int64 {
	var st unix.Statfs_t
	if err := unix.Statfs(s.mediaDir, &st); err != nil {
		slog.WarnContext(ctx, "media free space unavailable", "code", "status_statfs_failed")
		return nil
	}
	// big.Int keeps the unsigned-by-signed product exact; an absurd value is capped.
	total := new(big.Int).Mul(new(big.Int).SetUint64(st.Bavail), big.NewInt(st.Bsize))
	n := int64(math.MaxInt64)
	if total.IsInt64() {
		n = total.Int64()
	}
	return &n
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
