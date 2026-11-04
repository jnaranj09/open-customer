package status_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/status"
)

func clientErrorModule() (*status.Status, *clock.Fake) {
	fake := clock.NewFake(backupNow)
	return status.New(status.Options{Clock: fake}), fake
}

func validClientError() status.ClientError {
	return status.ClientError{Message: "boom", AppVersion: "1.2.3", Route: "/hoy"}
}

func TestLogClientErrorRateLimit(t *testing.T) {
	captureLog(t)
	s, fake := clientErrorModule()
	for i := range 10 {
		if err := s.LogClientError(t.Context(), "a", validClientError()); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}

	var rl problem.RateLimited
	err := s.LogClientError(t.Context(), "a", validClientError())
	if !errors.As(err, &rl) || rl.RetryAfter <= 0 {
		t.Fatalf("11th call err = %v, want RateLimited with RetryAfter > 0", err)
	}
	if err := s.LogClientError(t.Context(), "b", validClientError()); err != nil {
		t.Errorf("another session: %v", err)
	}
	fake.Advance(time.Minute)
	if err := s.LogClientError(t.Context(), "a", validClientError()); err != nil {
		t.Errorf("after a minute: %v", err)
	}
}

func TestLogClientErrorRejectsBadFields(t *testing.T) {
	captureLog(t)
	s, _ := clientErrorModule()
	for _, tc := range []struct {
		name  string
		mod   func(*status.ClientError)
		field string
	}{
		{"empty message", func(e *status.ClientError) { e.Message = "" }, "message"},
		{"2001-rune message", func(e *status.ClientError) { e.Message = strings.Repeat("é", 2001) }, "message"},
		{"empty app_version", func(e *status.ClientError) { e.AppVersion = "" }, "app_version"},
		{"201-rune route", func(e *status.ClientError) { e.Route = strings.Repeat("é", 201) }, "route"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := validClientError()
			tc.mod(&e)
			var v problem.Validation
			err := s.LogClientError(t.Context(), "a", e)
			if !errors.As(err, &v) || len(v.Errors) != 1 || v.Errors[0].Field != tc.field ||
				v.Errors[0].Message != problem.MsgClientErrorFieldInvalid {
				t.Fatalf("err = %v, want one validation error on %s", err, tc.field)
			}
		})
	}
}

func TestLogClientErrorLimitsAreInclusive(t *testing.T) {
	captureLog(t)
	s, _ := clientErrorModule()
	e := validClientError()
	e.Message = strings.Repeat("é", 2000)
	e.Route = strings.Repeat("r", 200)
	if err := s.LogClientError(t.Context(), "a", e); err != nil {
		t.Fatalf("fields at their limits: %v", err)
	}
}

func TestLogClientErrorLogsCode(t *testing.T) {
	logs := captureLog(t)
	s, _ := clientErrorModule()
	if err := s.LogClientError(t.Context(), "a", validClientError()); err != nil {
		t.Fatalf("LogClientError: %v", err)
	}
	got := logs.String()
	for _, want := range []string{`"level":"WARN"`, `"code":"client_error"`, `"route":"/hoy"`, `"app_version":"1.2.3"`, `"message":"boom"`} {
		if !strings.Contains(got, want) {
			t.Errorf("log lacks %s: %s", want, got)
		}
	}
}

func TestLogClientErrorCutsLongStack(t *testing.T) {
	logs := captureLog(t)
	s, _ := clientErrorModule()
	e := validClientError()
	stack := strings.Repeat("x", 8000) + "TAIL"
	e.Stack = &stack
	if err := s.LogClientError(t.Context(), "a", e); err != nil {
		t.Fatalf("LogClientError: %v", err)
	}
	got := logs.String()
	if !strings.Contains(got, strings.Repeat("x", 8000)) || strings.Contains(got, "TAIL") {
		t.Errorf("stack not cut to 8000 runes: %d bytes logged", len(got))
	}
}
