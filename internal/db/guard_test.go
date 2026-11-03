package db

import (
	"testing"
	"time"
)

func TestGuardBlocks(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	tests := []struct {
		name     string
		pending  bool
		version  int64
		lastCopy *time.Time
		want     bool
	}{
		{"blocks when pending, version 1 and the copy is 61 minutes old", true, 1, ago(61 * time.Minute), true},
		{"blocks when pending, version 1 and there is no copy", true, 1, nil, true},
		{"allows when the copy is 59 minutes old", true, 1, ago(59 * time.Minute), false},
		{"allows on a fresh database (version 0) with no copy", true, 0, nil, false},
		{"allows when nothing is pending, with no copy", false, 1, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := guardBlocks(tt.pending, tt.version, tt.lastCopy, now); got != tt.want {
				t.Fatalf("guardBlocks = %v, want %v", got, tt.want)
			}
		})
	}
}
