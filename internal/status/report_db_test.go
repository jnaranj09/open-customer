//go:build db

package status_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/push"
	"github.com/jnaranj09/open-customer/internal/scheduler"
	"github.com/jnaranj09/open-customer/internal/status"
)

type reportFixture struct {
	pool  *pgxpool.Pool
	clock *clock.Fake
	timer *scheduler.Scheduler
	s     *status.Status
}

func newReportFixture(t *testing.T, mediaDir string) reportFixture {
	t.Helper()
	pool := dbtest.Pool(t)
	fake := clock.NewFake(backupNow)
	timer := scheduler.New(scheduler.Options{Clock: fake, Pool: pool})
	s := status.New(status.Options{
		Version: "v-test", Clock: fake, MediaDir: mediaDir,
		BackupStatusFile: filepath.Join(t.TempDir(), "status.json"),
		Push:             push.New(push.Options{Clock: fake, Subject: "mailto:dev@example.com"}),
		Scheduler:        timer,
	})
	return reportFixture{pool: pool, clock: fake, timer: timer, s: s}
}

func (f reportFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func (f reportFixture) addSubscription(t *testing.T) {
	t.Helper()
	f.exec(t, `INSERT INTO users (id, username, password_hash) VALUES ('0192f0a0-0000-7000-8000-000000000001', 'ana', 'x')`)
	f.exec(t, `INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth)
		VALUES ('0192f0a0-0000-7000-8000-000000000002', '0192f0a0-0000-7000-8000-000000000001', 'https://push.example.com/1', 'k', 'a')`)
}

func TestReportMigrationVersionIsLatest(t *testing.T) {
	f := newReportFixture(t, t.TempDir())
	rep, err := f.s.Status(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	want := latestMigration(t)
	if rep.MigrationVersion != want || rep.Version != "v-test" {
		t.Errorf("version = %q, migration = %d; want v-test, %d", rep.Version, rep.MigrationVersion, want)
	}
}

// latestMigration is the highest number among the migration files the binary embeds (goose names
// them <number>_<name>.sql), so a new migration needs no change here.
func latestMigration(t *testing.T) int64 {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "db", "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("migration files: %v, %v", files, err)
	}
	var latest int64
	for _, f := range files {
		prefix, _, _ := strings.Cut(filepath.Base(f), "_")
		n, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			t.Fatalf("migration %s: %v", f, err)
		}
		latest = max(latest, n)
	}
	return latest
}

func TestReportHeartbeatAge(t *testing.T) {
	f := newReportFixture(t, t.TempDir())
	rep, err := f.s.Status(t.Context(), f.pool)
	if err != nil || rep.HeartbeatAgeS != nil {
		t.Fatalf("before the first tick: age = %v, err = %v; want nil", rep.HeartbeatAgeS, err)
	}

	if err := f.timer.Tick(t.Context()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	f.clock.Advance(90 * time.Second)
	rep, err = f.s.Status(t.Context(), f.pool)
	if err != nil || rep.HeartbeatAgeS == nil || *rep.HeartbeatAgeS != 90 {
		t.Fatalf("after a tick 90 s ago: age = %v, err = %v; want 90", rep.HeartbeatAgeS, err)
	}
}

func TestReportPushRed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		subs     bool
		lastPush string
		red      bool
	}{
		{"no subscription is red", false, "", true},
		{"a subscription and a push nobody received is red", true, `{"date":"2026-10-22","attempted":2,"delivered":0}`, true},
		{"a subscription and no push yet is fine", true, "", false},
		{"a delivered push is fine", true, `{"date":"2026-10-22","attempted":2,"delivered":1}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newReportFixture(t, t.TempDir())
			if tc.subs {
				f.addSubscription(t)
			}
			if tc.lastPush != "" {
				f.exec(t, `INSERT INTO scheduler_state (name, value) VALUES ('last_push', $1)`, tc.lastPush)
			}
			rep, err := f.s.Status(t.Context(), f.pool)
			if err != nil {
				t.Fatalf("Status: %v", err)
			}
			if rep.Push.Red != tc.red {
				t.Errorf("Push.Red = %v, want %v (%+v)", rep.Push.Red, tc.red, rep.Push)
			}
		})
	}
}

func TestReportPushCarriesLastPush(t *testing.T) {
	f := newReportFixture(t, t.TempDir())
	f.exec(t, `INSERT INTO scheduler_state (name, value) VALUES ('last_push', '{"date":"2026-10-22","attempted":3,"delivered":2}')`)
	rep, err := f.s.Status(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if lp := rep.Push.LastPush; lp == nil || lp.Date != "2026-10-22" || lp.Attempted != 3 || lp.Delivered != 2 {
		t.Errorf("LastPush = %+v", lp)
	}
}

func TestReportMediaFreeBytes(t *testing.T) {
	f := newReportFixture(t, t.TempDir())
	rep, err := f.s.Status(t.Context(), f.pool)
	if err != nil || rep.MediaFreeBytes == nil || *rep.MediaFreeBytes <= 0 {
		t.Fatalf("free = %v, err = %v; want > 0", rep.MediaFreeBytes, err)
	}
}

func TestReportMediaFreeBytesMissingDirIsNil(t *testing.T) {
	logs := captureLog(t)
	f := newReportFixture(t, filepath.Join(t.TempDir(), "gone"))
	rep, err := f.s.Status(t.Context(), f.pool)
	if err != nil || rep.MediaFreeBytes != nil {
		t.Fatalf("free = %v, err = %v; want nil", rep.MediaFreeBytes, err)
	}
	got := logs.String()
	if !strings.Contains(got, `"code":"status_statfs_failed"`) || strings.Contains(got, "gone") {
		t.Errorf("log = %s; want the code and no path", got)
	}
}

func TestReportReadsBackupFile(t *testing.T) {
	f := newReportFixture(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "s.json")
	if err := os.WriteFile(path, []byte(`{"checked_at":"2026-10-22T11:59:00Z","last_copy_at":"2026-10-22T08:05:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := status.New(status.Options{Clock: f.clock, BackupStatusFile: path, MediaDir: t.TempDir(), Push: push.New(push.Options{Clock: f.clock}), Scheduler: f.timer})
	rep, err := s.Status(t.Context(), f.pool)
	if err != nil || rep.Backup.Red || rep.Backup.LastCopyAt == nil {
		t.Errorf("backup = %+v, err = %v; want a fresh, not red line", rep.Backup, err)
	}
}

func TestStatusDBErrorIsReturned(t *testing.T) {
	f := newReportFixture(t, t.TempDir())
	f.pool.Close()
	if _, err := f.s.Status(t.Context(), f.pool); err == nil {
		t.Fatal("Status on a closed pool returned no error")
	}
}
