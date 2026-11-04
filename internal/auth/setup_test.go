//go:build db

package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

func TestSetupNeeded(t *testing.T) {
	pool := dbtest.Pool(t)
	start := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	a := auth.New(auth.Options{Clock: clock.NewFake(start)})

	needed, err := a.SetupNeeded(t.Context(), pool)
	if err != nil || !needed {
		t.Fatalf("empty database: SetupNeeded = %v, %v; want true, nil", needed, err)
	}

	if err := sqlc.New(pool).InsertUser(t.Context(), sqlc.InsertUserParams{
		ID: uuid.New(), Username: "made-up-owner", PasswordHash: "x", CreatedAt: start,
	}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	needed, err = a.SetupNeeded(t.Context(), pool)
	if err != nil || needed {
		t.Fatalf("after a user insert: SetupNeeded = %v, %v; want false, nil", needed, err)
	}
}
