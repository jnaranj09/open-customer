package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/platform/config"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	// healthzURL is serve's own port on loopback: the Compose healthcheck runs inside the app
	// container (design doc item 19).
	healthzURL = "http://127.0.0.1" + listenAddr + "/healthz"
	// healthcheckTimeout bounds a server that accepts the connection but never answers.
	healthcheckTimeout = 3 * time.Second
)

// The admin subcommands (design doc item 19) print only their result, the temporary password or
// the setup link, to stdout, so a script can read it; every error goes to stderr. They never print
// the username they were given. They do not migrate: only serve does, behind the backup guard.

// resetPassword gives the user a temporary password, prints it and exits 0. An unknown or
// malformed username exits 1.
func resetPassword(ctx context.Context, getenv func(string) string, username string, stdout, stderr io.Writer) int {
	_, pool, authModule, ok := openAdmin(ctx, getenv, "reset-password", stderr)
	if !ok {
		return 1
	}
	defer pool.Close()
	var temp string
	err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		if temp, err = authModule.ResetPassword(ctx, tx, username); err != nil {
			return fmt.Errorf("reset the password: %w", err)
		}
		return nil
	})
	switch {
	case errors.As(err, new(problem.NotFound)):
		printf(stderr, "reset-password: no such user\n")
		return 1
	case errors.As(err, new(problem.Validation)):
		printf(stderr, "reset-password: invalid username\n")
		return 1
	case err != nil:
		printf(stderr, "reset-password: %v\n", err)
		return 1
	}
	printf(stdout, "%s\n", temp)
	return 0
}

// setupLink prints a new setup link and exits 0 while no user exists; once one does it exits 1.
func setupLink(ctx context.Context, getenv func(string) string, stdout, stderr io.Writer) int {
	cfg, pool, authModule, ok := openAdmin(ctx, getenv, "setup-link", stderr)
	if !ok {
		return 1
	}
	defer pool.Close()
	needed, err := authModule.SetupNeeded(ctx, pool)
	if err != nil {
		printf(stderr, "setup-link: %v\n", err)
		return 1
	}
	if !needed {
		printf(stderr, "setup-link: setup is already complete\n")
		return 1
	}
	link, err := newSetupLink(ctx, cfg.PublicURL, pool, authModule)
	switch {
	case errors.As(err, new(problem.SetupClosed)):
		// A setup finished between the check and the new token.
		printf(stderr, "setup-link: setup is already complete\n")
		return 1
	case err != nil:
		printf(stderr, "setup-link: %v\n", err)
		return 1
	}
	printf(stdout, "%s\n", link)
	return 0
}

// openAdmin is the shared start of the admin subcommands: it loads the config, opens the pool and
// builds the auth module. A database with pending migrations is refused (OP18): an admin command
// must not migrate past the backup guard, and an old schema may not have what it needs. On failure
// it has printed why and closed the pool.
func openAdmin(ctx context.Context, getenv func(string) string, name string, stderr io.Writer) (config.Config, *pgxpool.Pool, *auth.Auth, bool) {
	cfg, err := config.Load(getenv)
	if err != nil {
		printf(stderr, "%v\n", err)
		return config.Config{}, nil, nil, false
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		printf(stderr, "%s: %v\n", name, err)
		return config.Config{}, nil, nil, false
	}
	pending, err := db.HasPending(ctx, pool)
	if err != nil {
		pool.Close()
		printf(stderr, "%s: %v\n", name, err)
		return config.Config{}, nil, nil, false
	}
	if pending {
		pool.Close()
		printf(stderr, "%s: migrations pending; start serve first\n", name)
		return config.Config{}, nil, nil, false
	}
	_, _, authModule, err := newModules(cfg)
	if err != nil {
		pool.Close()
		printf(stderr, "%s: %v\n", name, err)
		return config.Config{}, nil, nil, false
	}
	return cfg, pool, authModule, true
}

// newSetupLink makes a new setup token and returns the link <PUBLIC_URL>/setup#<token> ("Auth,
// sessions and handover"). NewSetupToken runs in its own transaction because the setup lock it
// takes lasts until that transaction ends. The link is a secret: callers print it, never log it.
func newSetupLink(ctx context.Context, publicURL *url.URL, pool *pgxpool.Pool, authModule *auth.Auth) (string, error) {
	var token string
	err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		if token, err = authModule.NewSetupToken(ctx, tx); err != nil {
			return fmt.Errorf("new setup token: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("setup link: %w", err)
	}
	return publicURL.String() + "/setup#" + token, nil
}

// healthcheck calls GET url and returns 0 on a 200 answer, otherwise 1 with the reason on stderr.
// It needs no config, so it runs in the distroless image with no environment and no shell.
func healthcheck(ctx context.Context, url string, stderr io.Writer) int {
	ctx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		printf(stderr, "healthcheck: %v\n", err)
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		printf(stderr, "healthcheck: %v\n", err)
		return 1
	}
	if err := resp.Body.Close(); err != nil {
		// The status already arrived; a close error does not change the answer.
		printf(stderr, "healthcheck: close the body: %v\n", err)
	}
	if resp.StatusCode != http.StatusOK {
		printf(stderr, "healthcheck: status %d\n", resp.StatusCode)
		return 1
	}
	return 0
}
