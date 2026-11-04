package status_test

import (
	"bytes"
	"log/slog"
	"testing"
)

// captureLog routes the default logger to a buffer for the test and restores it afterwards. Tests
// that use it never run in parallel.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}
