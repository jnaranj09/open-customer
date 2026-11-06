package auth_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

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

// A LoginAttempt lives across the handler's transaction; formatting or logging one, however a
// future debug line does it, must never show the password.
func TestLoginAttemptNeverShowsThePassword(t *testing.T) {
	t.Parallel()
	const password = "made-up-secret-password"
	a := auth.NewForTest(t, auth.Options{Clock: clock.NewFake(time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))})
	attempt, err := a.BeginLogin(auth.LoginInput{Username: "made-up-owner", Password: password}, netip.MustParseAddr("192.0.2.40"))
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	var logged bytes.Buffer
	slog.New(slog.NewJSONHandler(&logged, nil)).Info("attempt", "attempt", attempt)
	slog.New(slog.NewTextHandler(&logged, nil)).Info("attempt", "attempt", attempt)
	for _, out := range []string{
		fmt.Sprintf("%v", attempt), fmt.Sprintf("%+v", attempt), fmt.Sprintf("%#v", attempt), logged.String(),
	} {
		if strings.Contains(out, password) {
			t.Fatalf("a formatted attempt shows the password: %s", out)
		}
	}
}
