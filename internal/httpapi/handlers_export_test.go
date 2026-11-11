//go:build db

package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
)

const exportPath = "/api/v1/export"

// failingQueryTx is a real transaction whose queries go through dbtest.FailingDB.
type failingQueryTx struct {
	pgx.Tx
	f dbtest.FailingDB
}

func (tx failingQueryTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.f.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return rows, nil
}

func (tx failingQueryTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.f.QueryRow(ctx, sql, args...)
}

func (tx failingQueryTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.f.Exec(ctx, sql, args...)
	if err != nil {
		return tag, fmt.Errorf("exec: %w", err)
	}
	return tag, nil
}

// failingQueries begins real transactions whose query number failAt fails; calls counts them.
type failingQueries struct {
	pool   *pgxpool.Pool
	failAt int
	calls  *int
}

func (b failingQueries) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, err := b.pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	return failingQueryTx{Tx: tx, f: dbtest.FailingDB{DBTX: tx, FailAt: b.failAt, Calls: b.calls}}, nil
}

// zipEntryNames opens body as a ZIP and returns its entry names.
func zipEntryNames(t *testing.T, body []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("body is not a ZIP: %v", err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

func TestExportOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	createClient(t, app, c, clientBody)
	// 2026-03-02 23:30 UTC is already 2026-03-03 in the business zone (Europe/Madrid).
	app.clock.Set(time.Date(2026, 3, 2, 23, 30, 0, 0, time.UTC))

	rec := sendAs(app, c, http.MethodGet, exportPath, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%q)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", got)
	}
	if got, want := rec.Header().Get("Content-Disposition"), `attachment; filename="open-customer-2026-03-03.zip"`; got != want {
		t.Errorf("Content-Disposition = %q, want %q", got, want)
	}
	if got := zipEntryNames(t, rec.Body.Bytes()); len(got) != 5 {
		t.Errorf("entries = %v, want 5", got)
	}
}

func TestExportNeedsSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, send(app.handler, http.MethodGet, exportPath, testOrigin, "", nil))
}

func TestExportFailsBeforeStreamIsProblem(t *testing.T) {
	_, pool, c := signedInOwner(t)
	sink := &lockedBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(sink, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	// Query 6 is the exporter's first page of clients; the session, the handler's settings read and
	// the exporter's settings and attributes come before it. The err assertion below proves which
	// query failed. The ZIP writer buffers, so no byte has reached the client yet.
	var calls int
	app := newTestApp(t, testServerConfig{pool: pool, beginner: failingQueries{pool: pool, failAt: 6, calls: &calls}})

	rec := sendAs(app, c, http.MethodGet, exportPath, "")

	if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
		t.Fatalf("type = %q, want internal", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("Content-Disposition = %q on a problem response, want none", got)
	}
	line := lineWith(sink.lines(t), "code", "internal")
	if line == nil {
		t.Fatalf("no code=internal log line")
	}
	if got, _ := line["err"].(string); !strings.Contains(got, "export data: ") || !strings.Contains(got, "export clients: ") {
		t.Errorf("err = %q, want the failure inside the export's clients query", got)
	}
}

// lockedBuffer is a log sink a server goroutine writes while the test reads.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.buf.Write(p)
	if err != nil {
		return n, fmt.Errorf("buffer: %w", err)
	}
	return n, nil
}

func (b *lockedBuffer) lines(t *testing.T) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	return logLines(t, &b.buf)
}

// seedBulkyClients inserts clients whose notes do not compress, so the ZIP is far past the writer's
// buffers long before it ends.
func seedBulkyClients(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `INSERT INTO clients (id, name, notes)
		SELECT gen_random_uuid(), 'Ana Rojas', md5(random()::text) || md5(random()::text) FROM generate_series(1, 3000)`)
	if err != nil {
		t.Fatalf("seed clients: %v", err)
	}
}

func TestExportAbortsAfterStreamStarts(t *testing.T) {
	_, pool, c := signedInOwner(t)
	seedBulkyClients(t, pool)
	// A clean run tells how many queries an export has; the run that fails the last one has
	// streamed everything before it.
	var total int
	clean := httptest.NewServer(newTestApp(t, testServerConfig{pool: pool, beginner: failingQueries{pool: pool, calls: &total}}).handler)
	defer clean.Close()
	if body := getExport(t, clean.URL, c); body.err != nil || len(zipEntryNames(t, body.bytes)) != 5 {
		t.Fatalf("clean export: err %v", body.err)
	}
	sink := &lockedBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(sink, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	var calls int
	srv := httptest.NewServer(newTestApp(t, testServerConfig{pool: pool, beginner: failingQueries{pool: pool, failAt: total, calls: &calls}}).handler)
	defer srv.Close()

	got := getExport(t, srv.URL, c)

	if got.status != http.StatusOK {
		t.Errorf("status = %d, want 200 (the stream had started)", got.status)
	}
	if got.err == nil {
		t.Fatalf("read the whole body (%d bytes) with no error, want a broken download", len(got.bytes))
	}
	line := lineWith(sink.lines(t), "code", "export_aborted")
	if line == nil {
		t.Fatalf("no code=export_aborted log line")
	}
	if id, _ := line["request_id"].(string); id == "" {
		t.Errorf("export_aborted line has no request_id: %v", line)
	}
}

type exportResult struct {
	status int
	bytes  []byte
	err    error
}

// getExport downloads the export from a running server with the session cookie.
func getExport(t *testing.T, base string, c *http.Cookie) exportResult {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+exportPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(c)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return exportResult{err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return exportResult{status: resp.StatusCode, bytes: body, err: err}
}
