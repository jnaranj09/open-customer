// Package dbtest is the shared database test helper: one Postgres container per test process,
// and a fresh database per test. It is a plain package (no build tag) so the db-tagged tests of
// every module can import it; only tests call it.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/jnaranj09/open-customer/internal/db"
)

const (
	adminUser     = "postgres"
	adminPassword = "test"
	adminDB       = "postgres"
	templateName  = "template_migrated"
)

// Conn describes a database the caller can connect to.
type Conn struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	URL      string
}

type server struct {
	host, port string
	err        error
}

var (
	serverOnce sync.Once
	srv        server

	templateOnce sync.Once
	templateErr  error

	// Postgres refuses CREATE DATABASE ... TEMPLATE while another session is copying or using the
	// same template, so the copies are made one at a time.
	copyMu sync.Mutex
)

// startServer starts the process-wide container. The image comes from PG_IMAGE, which make exports.
func startServer(t *testing.T) server {
	t.Helper()
	image := os.Getenv("PG_IMAGE")
	if image == "" {
		t.Fatal("PG_IMAGE not set; run through make test-db")
	}
	serverOnce.Do(func() { srv = start(image) })
	if srv.err != nil {
		t.Fatalf("start Postgres container: %v", srv.err)
	}
	return srv
}

func start(image string) server {
	ctx := context.Background()
	c, err := postgres.Run(ctx, image,
		postgres.WithUsername(adminUser),
		postgres.WithPassword(adminPassword),
		postgres.WithDatabase(adminDB),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		return server{err: fmt.Errorf("run container: %w", err)}
	}
	host, err := c.Host(ctx)
	if err != nil {
		return server{err: fmt.Errorf("container host: %w", err)}
	}
	port, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return server{err: fmt.Errorf("container port: %w", err)}
	}
	return server{host: host, port: port.Port()}
}

func (s server) conn(name string) Conn {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(adminUser, adminPassword),
		Host:     net.JoinHostPort(s.host, s.port),
		Path:     "/" + name,
		RawQuery: "sslmode=disable",
	}
	return Conn{Host: s.host, Port: s.port, User: adminUser, Password: adminPassword, Name: name, URL: u.String()}
}

// admin runs one statement on the maintenance database.
func (s server) admin(ctx context.Context, sql string) (err error) {
	c, err := pgx.Connect(ctx, s.conn(adminDB).URL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() {
		if cerr := c.Close(ctx); cerr != nil && err == nil {
			err = fmt.Errorf("close: %w", cerr)
		}
	}()
	if _, err := c.Exec(ctx, sql); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}

// newDatabase creates an empty database (or a copy of template when template is not empty) and
// drops it when the test ends.
func newDatabase(t *testing.T, s server, template string) Conn {
	t.Helper()
	ctx := t.Context()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("random database name: %v", err)
	}
	name := "t_" + hex.EncodeToString(b[:])
	create := "CREATE DATABASE " + name
	if template != "" {
		create += " TEMPLATE " + template
	}
	if template != "" {
		copyMu.Lock()
		defer copyMu.Unlock()
	}
	if err := s.admin(ctx, create); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		// t.Context() is cancelled before cleanups run.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.admin(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop database: %v", err)
		}
	})
	return s.conn(name)
}

// Empty returns a fresh database with no tables.
func Empty(t *testing.T) Conn {
	t.Helper()
	return newDatabase(t, startServer(t), "")
}

// Migrated returns a fresh, fully migrated database. The migrations run once per process into a
// template; each test gets a copy.
func Migrated(t *testing.T) Conn {
	t.Helper()
	s := startServer(t)
	templateOnce.Do(func() { templateErr = buildTemplate(s) })
	if templateErr != nil {
		t.Fatalf("build migrated template: %v", templateErr)
	}
	return newDatabase(t, s, templateName)
}

// Pool returns a pool on a fresh, fully migrated database (see Migrated).
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	c := Migrated(t)
	pool, err := db.Open(t.Context(), c.URL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func buildTemplate(s server) error {
	ctx := context.Background()
	if err := s.admin(ctx, "CREATE DATABASE "+templateName); err != nil {
		return err
	}
	pool, err := db.Open(ctx, s.conn(templateName).URL)
	if err != nil {
		return fmt.Errorf("open template: %w", err)
	}
	// The guard is for upgrades; the template is a fresh database. Clock stays nil because the
	// guard is skipped.
	err = db.Migrate(ctx, pool, db.MigrateOptions{AllowWithoutBackup: true})
	// A template cannot be copied while it has connections.
	pool.Close()
	if err != nil {
		return fmt.Errorf("migrate template: %w", err)
	}
	return nil
}
