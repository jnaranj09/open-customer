package auth

import (
	"context"
	"testing"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

// NewForTest is the one test constructor for both test packages here. A nil SettingsCreator or
// Seeder in opts becomes a no-op, and clk is the clock; a New error fails the test.
func NewForTest(t *testing.T, clk clock.Clock, opts Options) *Auth {
	t.Helper()
	opts.Clock = clk
	if opts.SettingsCreator == nil {
		opts.SettingsCreator = func(context.Context, db.DBTX, SetupSettings) error { return nil }
	}
	if opts.Seeder == nil {
		opts.Seeder = func(context.Context, db.DBTX, string) error { return nil }
	}
	a, err := New(opts)
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	return a
}
