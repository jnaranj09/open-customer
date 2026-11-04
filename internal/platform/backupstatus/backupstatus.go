// Package backupstatus reads the status.json file the backup container writes. It is the one
// parse site for BACKUP_STATUS_FILE: the migration guard and the status screen both call Read.
package backupstatus

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

var (
	// ErrMissing means the file does not exist.
	ErrMissing = errors.New("backup status file missing")
	// ErrInvalid means the file could not be read or its content is not a valid status.
	ErrInvalid = errors.New("backup status file invalid")
)

// Status is the parsed status.json. A nil field was null in the file.
type Status struct {
	CheckedAt      *time.Time
	LastDumpAt     *time.Time
	LastCopyAt     *time.Time
	LastErrorAt    *time.Time
	LastSnapshotID *string
	LastError      *string
}

// wire is the file as written; times stay text until parsed. Unknown fields are ignored.
type wire struct {
	CheckedAt      *string `json:"checked_at"`
	LastDumpAt     *string `json:"last_dump_at"`
	LastCopyAt     *string `json:"last_copy_at"`
	LastErrorAt    *string `json:"last_error_at"`
	LastSnapshotID *string `json:"last_snapshot_id"`
	LastError      *string `json:"last_error"`
}

// Read parses the file at path. A missing file is ErrMissing; an unreadable file, bad JSON, a
// wrong type or a time that is not RFC 3339 is ErrInvalid. Errors never carry the path or any
// file content, so callers can log them.
func Read(path string) (Status, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return Status{}, ErrMissing
	}
	if err != nil {
		return Status{}, fmt.Errorf("%w: read failed", ErrInvalid)
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return Status{}, fmt.Errorf("%w: not the expected JSON", ErrInvalid)
	}
	var s Status
	for _, f := range []struct {
		raw *string
		dst **time.Time
	}{
		{w.CheckedAt, &s.CheckedAt},
		{w.LastDumpAt, &s.LastDumpAt},
		{w.LastCopyAt, &s.LastCopyAt},
		{w.LastErrorAt, &s.LastErrorAt},
	} {
		if f.raw == nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, *f.raw)
		if err != nil {
			return Status{}, fmt.Errorf("%w: bad time", ErrInvalid)
		}
		t = t.UTC()
		*f.dst = &t
	}
	s.LastSnapshotID = w.LastSnapshotID
	s.LastError = w.LastError
	return s, nil
}
