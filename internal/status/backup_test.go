package status_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/status"
)

var backupNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func backupModule(t *testing.T, content *string) *status.Status {
	t.Helper()
	path := filepath.Join(t.TempDir(), "status.json")
	if content != nil {
		if err := os.WriteFile(path, []byte(*content), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	return status.New(status.Options{Clock: clock.NewFake(backupNow), BackupStatusFile: path})
}

func TestBackupMissingFileIsRed(t *testing.T) {
	corrupt := "{not json"
	for name, content := range map[string]*string{"missing": nil, "corrupt": &corrupt} {
		t.Run(name, func(t *testing.T) {
			b := backupModule(t, content).Backup()
			if !b.Red {
				t.Error("Red = false, want true")
			}
			if b.CheckedAt != nil || b.LastDumpAt != nil || b.LastCopyAt != nil || b.LastError != nil {
				t.Errorf("times or error set on an unreadable file: %+v", b)
			}
		})
	}
}

func TestBackupRedRules(t *testing.T) {
	const (
		fresh      = "2026-10-22T11:30:00Z"
		copy47h    = "2026-10-20T13:00:00Z"
		copy49h    = "2026-10-20T11:00:00Z"
		checked3h  = "2026-10-22T09:00:00Z"
		backupForm = "2026-10-22T08:05:00Z"
	)
	file := func(checked, copied, lastErr string) string {
		out := `{"checked_at":` + quoteOrNull(checked) + `,"last_copy_at":` + quoteOrNull(copied) +
			`,"last_error":` + quoteOrNull(lastErr) + `}`
		return out
	}
	for _, tc := range []struct {
		name    string
		content string
		red     bool
	}{
		{"copy 49 h old is red", file(fresh, copy49h, ""), true},
		{"copy 47 h old is fine", file(fresh, copy47h, ""), false},
		{"last_error set is red", file(fresh, copy47h, "disk full"), true},
		{"checked_at 3 h old is red", file(checked3h, copy47h, ""), true},
		{"checked_at missing is red", file("", copy47h, ""), true},
		{"last_copy_at missing is red", file(fresh, "", ""), true},
		{"the backup.sh time form parses", file("2026-10-22T11:59:00Z", backupForm, ""), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := backupModule(t, &tc.content).Backup()
			if b.Red != tc.red {
				t.Errorf("Red = %v, want %v", b.Red, tc.red)
			}
		})
	}
}

func TestBackupRedMatchesBackup(t *testing.T) {
	if !backupModule(t, nil).BackupRed() {
		t.Error("BackupRed = false on a missing file, want true")
	}
}

func quoteOrNull(s string) string {
	if s == "" {
		return "null"
	}
	return `"` + s + `"`
}
