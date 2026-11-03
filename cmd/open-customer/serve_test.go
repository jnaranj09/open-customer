//go:build db

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/config"
)

// These tests are not parallel: serve calls slog.SetDefault, which is process-wide.

// lockedBuffer is a bytes.Buffer the server goroutines can write while the test reads it.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Write(p) // bytes.Buffer.Write only ever returns len(p), nil
	return len(p), nil
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// serveConfig loads the config serve would see for a database on host:port.
func serveConfig(t *testing.T, c dbtest.Conn) config.Config {
	t.Helper()
	env := map[string]string{
		"PUBLIC_URL":    "http://localhost:5173",
		"DB_HOST":       c.Host,
		"DB_PORT":       c.Port,
		"DB_USER":       c.User,
		"DB_PASSWORD":   c.Password,
		"DB_NAME":       c.Name,
		"VAPID_SUBJECT": "mailto:dev@example.test",
		// No backup has run; a fresh database migrates anyway, with the guard on.
		"BACKUP_STATUS_FILE": filepath.Join(t.TempDir(), "status.json"),
	}
	cfg, err := config.Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

func get(t *testing.T, client *http.Client, url string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close body: %v", err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(body)
}

func TestServeMigratesEmptyDBAndAnswers(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	cfg := serveConfig(t, dbtest.Empty(t))
	ln := listen(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var stdout, stderr lockedBuffer
	done := make(chan int, 1)
	// No wait for "ready": the listener is already bound, so the backlog holds each request
	// until serveOn has migrated and starts serving.
	go func() { done <- serveOn(ctx, cfg, ln, &stdout, &stderr) }()

	client := &http.Client{Timeout: 30 * time.Second}
	base := "http://" + ln.Addr().String()

	if resp, body := get(t, client, base+"/readyz"); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /readyz = %d %q, want 200 (stderr %q)", resp.StatusCode, body, stderr.String())
	}
	if resp, body := get(t, client, base+"/api/v1/setup/status"); resp.StatusCode != http.StatusOK ||
		strings.TrimSpace(body) != `{"needs_setup":true}` {
		t.Fatalf("GET /api/v1/setup/status = %d %q, want 200 {\"needs_setup\":true}", resp.StatusCode, body)
	}
	resp, body := get(t, client, base+"/")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("GET / = %d %q (Content-Type %q), want 200 HTML", resp.StatusCode, body, resp.Header.Get("Content-Type"))
	}

	// A line through slog.Default (problem.Write, migrate, status use it) lands in the JSON log.
	slog.Info("default logger probe")
	if !hasJSONLine(stdout.String(), "default logger probe") {
		t.Errorf("stdout has no JSON line with msg %q:\n%s", "default logger probe", stdout.String())
	}

	client.CloseIdleConnections()
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("serveOn returned %d after cancel, want 0 (stderr %q)", code, stderr.String())
	}
}

func hasJSONLine(out, msg string) bool {
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		var line struct {
			Msg string `json:"msg"`
		}
		if json.Unmarshal(sc.Bytes(), &line) == nil && line.Msg == msg {
			return true
		}
	}
	return false
}

func TestServeExitsOneWhenMigrateFails(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	// A port nothing listens on: bind one, then close it.
	closed := listen(t)
	port := strconv.Itoa(closed.Addr().(*net.TCPAddr).Port)
	if err := closed.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	cfg := serveConfig(t, dbtest.Conn{Host: "127.0.0.1", Port: port, User: "open_customer", Password: "pw", Name: "open_customer"})
	ln := listen(t)
	var stdout, stderr lockedBuffer

	if code := serveOn(t.Context(), cfg, ln, &stdout, &stderr); code != 1 {
		t.Fatalf("serveOn = %d, want 1", code)
	}
	if stderr.String() == "" {
		t.Error("stderr is empty, want the database error")
	}
	// serveOn owns the listener: on the way out it is closed.
	if c, err := ln.Accept(); err == nil {
		t.Errorf("listener still accepts after serveOn failed (conn %v)", c.LocalAddr())
	}
}
