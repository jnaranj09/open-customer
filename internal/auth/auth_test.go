package auth_test

import (
	"context"
	"testing"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

func TestNewRejectsNilCreator(t *testing.T) {
	t.Parallel()
	creator := func(context.Context, db.DBTX, auth.SetupSettings) error { return nil }
	t.Run("a nil settings creator is an error", func(t *testing.T) {
		t.Parallel()
		a, err := auth.New(auth.Options{Clock: clock.Real{}})
		if err == nil || a != nil {
			t.Fatalf("New = %v, %v; want nil and an error", a, err)
		}
	})
	t.Run("a set settings creator is accepted", func(t *testing.T) {
		t.Parallel()
		a, err := auth.New(auth.Options{Clock: clock.Real{}, SettingsCreator: creator})
		if err != nil || a == nil {
			t.Fatalf("New = %v, %v; want an Auth and nil", a, err)
		}
	})
}
