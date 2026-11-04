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
	"regexp"
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

// connEnv is the environment serve and the admin commands see for the database c.
func connEnv(t *testing.T, c dbtest.Conn) func(string) string {
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
	return func(k string) string { return env[k] }
}

// serveConfig loads the config serve would see for the database c.
func serveConfig(t *testing.T, c dbtest.Conn) config.Config {
	t.Helper()
	cfg, err := config.Load(connEnv(t, c))
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

// runningServe is a serveOn running in the background on a test listener.
type runningServe struct {
	base           string
	client         *http.Client
	stdout, stderr *lockedBuffer
	cancel         context.CancelFunc
	done           chan int
}

// startServe runs serveOn for cfg and waits until /readyz answers 200, so startup (migrations and
// the setup link) has finished. It restores slog.Default when the test ends.
func startServe(t *testing.T, cfg config.Config) *runningServe {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	ln := listen(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	s := &runningServe{
		base:   "http://" + ln.Addr().String(),
		client: &http.Client{Timeout: 30 * time.Second},
		stdout: &lockedBuffer{}, stderr: &lockedBuffer{},
		cancel: cancel, done: make(chan int, 1),
	}
	// No wait for "ready": the listener is already bound, so the backlog holds each request
	// until serveOn has migrated and starts serving.
	go func() { s.done <- serveOn(ctx, cfg, ln, s.stdout, s.stderr) }()
	if resp, body := get(t, s.client, s.base+"/readyz"); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /readyz = %d %q, want 200 (stderr %q)", resp.StatusCode, body, s.stderr.String())
	}
	return s
}

// stop cancels serveOn and checks it shut down gracefully.
func (s *runningServe) stop(t *testing.T) {
	t.Helper()
	s.client.CloseIdleConnections()
	s.cancel()
	if code := <-s.done; code != 0 {
		t.Fatalf("serveOn returned %d after cancel, want 0 (stderr %q)", code, s.stderr.String())
	}
}

func TestServeMigratesEmptyDBAndAnswers(t *testing.T) {
	s := startServe(t, serveConfig(t, dbtest.Empty(t)))

	if resp, body := get(t, s.client, s.base+"/api/v1/setup/status"); resp.StatusCode != http.StatusOK ||
		strings.TrimSpace(body) != `{"needs_setup":true}` {
		t.Fatalf("GET /api/v1/setup/status = %d %q, want 200 {\"needs_setup\":true}", resp.StatusCode, body)
	}
	// The manifest and icon routes sit outside /api/v1, before the SPA fallback.
	if resp, body := get(t, s.client, s.base+"/manifest.webmanifest"); resp.StatusCode != http.StatusOK ||
		resp.Header.Get("Content-Type") != "application/manifest+json" {
		t.Fatalf("GET /manifest.webmanifest = %d %q, want 200 application/manifest+json", resp.StatusCode, body)
	}
	resp, body := get(t, s.client, s.base+"/")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("GET / = %d %q (Content-Type %q), want 200 HTML", resp.StatusCode, body, resp.Header.Get("Content-Type"))
	}

	// A line through slog.Default (problem.Write, migrate, status use it) lands in the JSON log.
	slog.Info("default logger probe")
	if !hasJSONLine(s.stdout.String(), "default logger probe") {
		t.Errorf("stdout has no JSON line with msg %q:\n%s", "default logger probe", s.stdout.String())
	}
	s.stop(t)
}

func TestServePrintsSetupLinkOnEmptyDB(t *testing.T) {
	s := startServe(t, serveConfig(t, dbtest.Empty(t)))
	s.stop(t)

	link := regexp.MustCompile(`(?m)^Setup: http://localhost:5173/setup#[A-Za-z0-9_-]{43}$`)
	if !link.MatchString(s.stdout.String()) {
		t.Errorf("stdout has no plain setup link line:\n%s", s.stdout.String())
	}
}

func TestServeNoSetupLinkWhenUserExists(t *testing.T) {
	s := startServe(t, serveConfig(t, withOwner(t)))
	s.stop(t)

	if strings.Contains(s.stdout.String(), "/setup#") {
		t.Errorf("stdout has a setup link although a user exists:\n%s", s.stdout.String())
	}
}

// vapidKeys reads the two VAPID rows of server_keys from the database c, name to value.
func vapidKeys(t *testing.T, c dbtest.Conn) map[string]string {
	t.Helper()
	pool := openPool(t, c)
	rows, err := pool.Query(t.Context(), `SELECT name, value FROM server_keys WHERE name IN ('vapid_public', 'vapid_private')`)
	if err != nil {
		t.Fatalf("query server_keys: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			t.Fatalf("scan server_keys: %v", err)
		}
		got[name] = value
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read server_keys: %v", err)
	}
	return got
}

func TestServeCreatesVAPIDKeys(t *testing.T) {
	c := dbtest.Empty(t)
	cfg := serveConfig(t, c)
	first := startServe(t, cfg)
	first.stop(t)
	created := vapidKeys(t, c)
	if created["vapid_public"] == "" || created["vapid_private"] == "" {
		t.Fatalf("server_keys after start = %v, want vapid_public and vapid_private", created)
	}

	second := startServe(t, cfg)
	second.stop(t)
	if kept := vapidKeys(t, c); kept["vapid_public"] != created["vapid_public"] || kept["vapid_private"] != created["vapid_private"] {
		t.Errorf("server_keys after a restart differ from the first start: the keys were regenerated")
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
