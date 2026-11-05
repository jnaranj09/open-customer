package exporter_test

import (
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/exporter"
)

func TestFileNameUsesTheDay(t *testing.T) {
	day := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	if got := exporter.FileName(day); got != "open-customer-2026-10-24.zip" {
		t.Fatalf("FileName = %q", got)
	}
}
