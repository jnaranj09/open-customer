//go:build db

package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
)

func TestGetSetupStatusEmptyDB(t *testing.T) {
	rec := serve(newTestServer(t, dbtest.Pool(t)), http.MethodGet, "/api/v1/setup/status")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"needs_setup":true}` {
		t.Fatalf("got %d %q, want 200 {\"needs_setup\":true}", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if got := rec.Header().Get("X-App-Version"); got != testVersion {
		t.Fatalf("X-App-Version on a generated route = %q, want %q", got, testVersion)
	}
}

func TestGetSetupStatusWithUser(t *testing.T) {
	pool := dbtest.Pool(t)
	if err := sqlc.New(pool).InsertUser(t.Context(), sqlc.InsertUserParams{
		ID:           uuid.New(),
		Username:     "made-up-owner",
		PasswordHash: "x",
		CreatedAt:    time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	rec := serve(newTestServer(t, pool), http.MethodGet, "/api/v1/setup/status")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"needs_setup":false}` {
		t.Fatalf("got %d %q, want 200 {\"needs_setup\":false}", rec.Code, rec.Body.String())
	}
}

func TestGetSetupStatusDBDownIsInternalProblem(t *testing.T) {
	pool := dbtest.Pool(t)
	pool.Close()
	rec := serve(newTestServer(t, pool), http.MethodGet, "/api/v1/setup/status")
	if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
		t.Fatalf("type = %q, want internal", got)
	}
}

func TestHealthz(t *testing.T) {
	rec := serve(newTestServer(t, nil), http.MethodGet, "/healthz")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("got %d %q %q, want 200 text/plain ok", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
}

func TestReadyz(t *testing.T) {
	rec := serve(newTestServer(t, dbtest.Pool(t)), http.MethodGet, "/readyz")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("got %d %q %q, want 200 text/plain ok", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
}

func TestReadyzDBDownIs503(t *testing.T) {
	pool := dbtest.Pool(t)
	pool.Close()
	rec := serve(newTestServer(t, pool), http.MethodGet, "/readyz")
	if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != "not ready" {
		t.Fatalf("got %d %q, want 503 not ready", rec.Code, rec.Body.String())
	}
}

func TestGetSetupStatusAfterSetupHasAccent(t *testing.T) {
	app, _, _ := signedInOwner(t)
	// No cookie: the login screen asks before she is signed in.
	rec := serve(app.handler, http.MethodGet, "/api/v1/setup/status")
	if rec.Code != http.StatusOK ||
		strings.TrimSpace(rec.Body.String()) != `{"accent_color":"#aa3366","needs_setup":false}` {
		t.Fatalf("got %d %q, want 200 with the accent color", rec.Code, rec.Body.String())
	}
}
