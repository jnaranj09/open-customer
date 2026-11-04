//go:build db

package status_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/status"
)

// captureLog routes the default logger to a buffer for the test and restores it afterwards.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestHealthIsAliveWithoutDatabase(t *testing.T) {
	if !status.New("test").Health() {
		t.Fatal("Health = false, want true")
	}
}

func TestReadyTrueOnMigratedDB(t *testing.T) {
	logs := captureLog(t)
	pool := dbtest.Pool(t)

	if !status.New("test").Ready(t.Context(), pool) {
		t.Fatal("Ready = false on a migrated database, want true")
	}
	if logs.Len() != 0 {
		t.Fatalf("Ready logged on success: %s", logs)
	}
}

func TestReadyFalseWithPendingMigrations(t *testing.T) {
	logs := captureLog(t)
	pool, err := db.Open(t.Context(), dbtest.Empty(t).URL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if status.New("test").Ready(t.Context(), pool) {
		t.Fatal("Ready = true on an unmigrated database, want false")
	}
	if !strings.Contains(logs.String(), `"code":"ready_migrations_pending"`) {
		t.Fatalf("log lacks code=ready_migrations_pending: %s", logs)
	}
}

func TestReadyFalseWhenDBClosed(t *testing.T) {
	logs := captureLog(t)
	pool := dbtest.Pool(t)
	pool.Close()

	if status.New("test").Ready(t.Context(), pool) {
		t.Fatal("Ready = true on a closed pool, want false")
	}
	got := logs.String()
	if !strings.Contains(got, `"code":"ready_db_unreachable"`) {
		t.Fatalf("log lacks code=ready_db_unreachable: %s", got)
	}
	if strings.Contains(got, "closed pool") || strings.Contains(got, `"error"`) {
		t.Fatalf("log carries error text: %s", got)
	}
}
