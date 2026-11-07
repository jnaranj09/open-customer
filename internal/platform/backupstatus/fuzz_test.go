package backupstatus

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzBackupStatusRead checks Read on any file content: it never panics, and it returns either a
// Status with a nil error or ErrInvalid. It never returns ErrMissing for a file that exists, and an
// error never carries the path.
func FuzzBackupStatusRead(f *testing.F) {
	for _, seed := range []string{
		`{"checked_at":"2026-11-03T10:00:00Z","last_dump_at":"2026-11-03T09:29:00Z","last_copy_at":"2026-11-03T09:30:00Z","last_snapshot_id":"abc123","last_error":"copy: boom","last_error_at":"2026-11-02T08:00:00Z"}`,
		`{"checked_at":"2026-11-03T10:00:00Z","last_dump_at":null,"last_copy_at":null,"last_snapshot_id":null,"last_error":null,"last_error_at":null}`,
		`{"checked_at":"2026-11-03T10:00:00Z","future_field":{"a":1}}`,
		`{"checked_at":"yesterday"}`, `{"checked_at":5}`, `{"checked_at":"2026-11-03T10:00:00+99:00"}`,
		`{"last_error":"line\u0000nul"}`, `[]`, `null`, `{`, ``, "\x00", "\xff\xfe", `{"checked_at":"2026-11-03T10:00:00.123456789-07:00"}`,
	} {
		f.Add([]byte(seed))
	}
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, content []byte) {
		path := filepath.Join(dir, "status.json")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write input: %v", err)
		}
		_, err := Read(path)
		if err == nil {
			return
		}
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Read error = %v, want ErrInvalid", err)
		}
		if strings.Contains(err.Error(), dir) {
			t.Fatalf("error text names the path: %q", err.Error())
		}
	})
}
