//go:build db

package auth_test

import (
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

func TestSetupNeeded(t *testing.T) {
	pool := dbtest.Pool(t)
	a := auth.New(auth.Options{Clock: clock.NewFake(time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))})

	needed, err := a.SetupNeeded(t.Context(), pool)
	if err != nil || !needed {
		t.Fatalf("empty database: SetupNeeded = %v, %v; want true, nil", needed, err)
	}

	if _, err := pool.Exec(t.Context(),
		`INSERT INTO users (id, username, password_hash) VALUES (gen_random_uuid(), 'made-up-owner', 'x')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	needed, err = a.SetupNeeded(t.Context(), pool)
	if err != nil || needed {
		t.Fatalf("after a user insert: SetupNeeded = %v, %v; want false, nil", needed, err)
	}
}
