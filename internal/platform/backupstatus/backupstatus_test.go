package backupstatus

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestReadParsesTheBackupShTimeForm(t *testing.T) {
	t.Parallel()
	path := writeFile(t, `{"checked_at":"2026-11-03T10:00:00Z","last_dump_at":"2026-11-03T09:29:00Z",`+
		`"last_copy_at":"2026-11-03T09:30:00Z","last_snapshot_id":"abc123",`+
		`"last_error":"copy: boom","last_error_at":"2026-11-02T08:00:00Z"}`)

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	wantTimes := map[string]struct {
		got  *time.Time
		want time.Time
	}{
		"checked_at":    {got.CheckedAt, time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)},
		"last_dump_at":  {got.LastDumpAt, time.Date(2026, 11, 3, 9, 29, 0, 0, time.UTC)},
		"last_copy_at":  {got.LastCopyAt, time.Date(2026, 11, 3, 9, 30, 0, 0, time.UTC)},
		"last_error_at": {got.LastErrorAt, time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)},
	}
	for name, c := range wantTimes {
		if c.got == nil || !c.got.Equal(c.want) || c.got.Location() != time.UTC {
			t.Errorf("%s = %v, want %v in UTC", name, c.got, c.want)
		}
	}
	if got.LastSnapshotID == nil || *got.LastSnapshotID != "abc123" {
		t.Errorf("LastSnapshotID = %v, want abc123", got.LastSnapshotID)
	}
	if got.LastError == nil || *got.LastError != "copy: boom" {
		t.Errorf("LastError = %v, want copy: boom", got.LastError)
	}
}

func TestReadNullFieldsAreNil(t *testing.T) {
	t.Parallel()
	path := writeFile(t, `{"checked_at":"2026-11-03T10:00:00Z","last_dump_at":null,"last_copy_at":null,`+
		`"last_snapshot_id":null,"last_error":null,"last_error_at":null}`)

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if got.CheckedAt == nil {
		t.Error("CheckedAt is nil, want a time")
	}
	if got.LastDumpAt != nil || got.LastCopyAt != nil || got.LastErrorAt != nil ||
		got.LastSnapshotID != nil || got.LastError != nil {
		t.Errorf("null fields are not nil: %+v", got)
	}
}

func TestReadMissingFileIsErrMissing(t *testing.T) {
	t.Parallel()
	_, err := Read(filepath.Join(t.TempDir(), "absent.json"))
	if !errors.Is(err, ErrMissing) {
		t.Fatalf("error = %v, want ErrMissing", err)
	}
	if errors.Is(err, ErrInvalid) {
		t.Fatalf("a missing file must not be ErrInvalid: %v", err)
	}
}

func TestReadBadContentIsInvalid(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"malformed JSON":          `{"checked_at":`,
		"empty file":              ``,
		"not an object":           `[1,2]`,
		"trailing data":           `{} {}`,
		"unparsable time":         `{"last_copy_at":"yesterday"}`,
		"time as a number":        `{"last_copy_at":123}`,
		"text as a number":        `{"last_error":5}`,
		"snapshot id as an array": `{"last_snapshot_id":["a"]}`,
		"date without a zone":     `{"checked_at":"2026-11-03T10:00:00"}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Read(writeFile(t, content))
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
			if errors.Is(err, ErrMissing) {
				t.Fatalf("a bad file must not be ErrMissing: %v", err)
			}
		})
	}
}

func TestReadDirectoryIsInvalidNotMissing(t *testing.T) {
	t.Parallel()
	_, err := Read(t.TempDir())
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
}

func TestReadIgnoresUnknownFields(t *testing.T) {
	t.Parallel()
	got, err := Read(writeFile(t, `{"checked_at":"2026-11-03T10:00:00Z","future_field":{"a":1}}`))
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if got.CheckedAt == nil {
		t.Error("CheckedAt is nil, want a time")
	}
}

func TestReadErrorNeverQuotesContent(t *testing.T) {
	t.Parallel()
	const secret = "SENTINEL-VALUE-9f3a"
	contents := []string{
		`{"last_copy_at":"` + secret + `"}`,
		`{"last_error":["` + secret + `"]}`,
		`{"` + secret + `": }`,
	}
	for _, content := range contents {
		path := writeFile(t, content)
		_, err := Read(path)
		if err == nil {
			t.Fatalf("Read(%q) succeeded, want an error", content)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error quotes file content: %v", err)
		}
		if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), filepath.Dir(path)) {
			t.Errorf("error quotes the path: %v", err)
		}
	}
}

func TestReadMissingErrorNeverQuotesPath(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "absent.json")
	_, err := Read(path)
	if err == nil || strings.Contains(err.Error(), filepath.Dir(path)) {
		t.Fatalf("error = %v, want one without the path", err)
	}
}

func TestReadNormalizesOffsetTimesToUTC(t *testing.T) {
	t.Parallel()
	got, err := Read(writeFile(t, `{"last_copy_at":"2026-11-03T11:30:00+02:00"}`))
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	want := time.Date(2026, 11, 3, 9, 30, 0, 0, time.UTC)
	if got.LastCopyAt == nil || !got.LastCopyAt.Equal(want) || got.LastCopyAt.Location() != time.UTC {
		t.Fatalf("LastCopyAt = %v, want %v in UTC", got.LastCopyAt, want)
	}
}
