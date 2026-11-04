package auth_test

import (
	"context"
	"testing"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

func TestNewRejectsNilCreatorOrSeeder(t *testing.T) {
	t.Parallel()
	creator := func(context.Context, db.DBTX, auth.SetupSettings) error { return nil }
	seeder := func(context.Context, db.DBTX, string) error { return nil }
	cases := []struct {
		name string
		opts auth.Options
	}{
		{"a nil settings creator", auth.Options{Clock: clock.Real{}, Seeder: seeder}},
		{"a nil seeder", auth.Options{Clock: clock.Real{}, SettingsCreator: creator}},
		{"both nil", auth.Options{Clock: clock.Real{}}},
	}
	for _, tt := range cases {
		t.Run(tt.name+" is an error", func(t *testing.T) {
			t.Parallel()
			a, err := auth.New(tt.opts)
			if err == nil || a != nil {
				t.Fatalf("New = %v, %v; want nil and an error", a, err)
			}
		})
	}
	t.Run("both set is accepted", func(t *testing.T) {
		t.Parallel()
		a, err := auth.New(auth.Options{Clock: clock.Real{}, SettingsCreator: creator, Seeder: seeder})
		if err != nil || a == nil {
			t.Fatalf("New = %v, %v; want an Auth and nil", a, err)
		}
	})
}
