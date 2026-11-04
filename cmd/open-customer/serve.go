package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/httpapi"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/config"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/services"
	"github.com/jnaranj09/open-customer/internal/settings"
	"github.com/jnaranj09/open-customer/internal/status"
	"github.com/jnaranj09/open-customer/web"
)

const (
	// listenAddr is the app's port inside the Compose network (design doc item 18, step 7).
	listenAddr = ":8080"
	// shutdownTimeout is how long in-flight requests get after SIGTERM.
	shutdownTimeout = 10 * time.Second
	// readHeaderTimeout bounds a client that never finishes its request headers.
	readHeaderTimeout = 10 * time.Second
)

// serve runs the server (design doc item 18; P1 and P2 have steps 1, 2, 4, 5, 7 and 8) until ctx is
// done.
// It returns 0 after a graceful shutdown and 1 on a config, database, migration or listen error,
// which it prints to stderr.
func serve(ctx context.Context, getenv func(string) string, stdout, stderr io.Writer) int {
	cfg, err := config.Load(getenv)
	if err != nil {
		printf(stderr, "%v\n", err)
		return 1
	}
	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", listenAddr)
	if err != nil {
		printf(stderr, "serve: listen: %v\n", err)
		return 1
	}
	return serveOn(ctx, cfg, ln, stdout, stderr)
}

// serveOn is serve after config, on a listener the caller bound (tests use 127.0.0.1:0). It owns
// ln and closes it on every path.
func serveOn(ctx context.Context, cfg config.Config, ln net.Listener, stdout, stderr io.Writer) int {
	log := logging.New(stdout, cfg.LogLevel)
	// Every slog.Default() line (problem.Write, decodeJSON, migrate, status) goes to the JSON log.
	slog.SetDefault(log)

	srv, pool, err := startup(ctx, cfg, log, stdout)
	if err != nil {
		closeListener(ln)
		printf(stderr, "%v\n", err)
		return 1
	}
	defer pool.Close()

	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	log.InfoContext(ctx, "serving", "addr", ln.Addr().String(), "version", version)

	select {
	case err := <-served:
		// Serve only returns early on a listener failure.
		printf(stderr, "serve: %v\n", err)
		return 1
	case <-ctx.Done():
	}

	// ctx is already cancelled; the shutdown gets its own deadline.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		printf(stderr, "serve: shutdown: %v\n", err)
		return 1
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		printf(stderr, "serve: %v\n", err)
		return 1
	}
	log.InfoContext(ctx, "stopped")
	return 0
}

// startup opens the pool, runs the migrations behind the backup guard, prints the setup link while
// no user exists and builds the HTTP server. On an error the pool is already closed.
func startup(ctx context.Context, cfg config.Config, log *slog.Logger, stdout io.Writer) (*http.Server, *pgxpool.Pool, error) {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("serve: %w", err)
	}
	err = db.Migrate(ctx, pool, db.MigrateOptions{
		AllowWithoutBackup: cfg.AllowMigrateWithoutBackup,
		BackupStatusFile:   cfg.BackupStatusFile,
		Clock:              clock.Real{},
		Logger:             log,
	})
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("serve: %w", err)
	}
	mods, err := newModules(cfg)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("serve: %w", err)
	}
	if err := printSetupLink(ctx, cfg, pool, mods.auth, stdout); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("serve: %w", err)
	}
	static, err := fs.Sub(web.Build, "build")
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("serve: embedded web build: %w", err)
	}
	handler, err := httpapi.New(httpapi.Options{
		Pool:           pool,
		Version:        version,
		PublicURL:      cfg.PublicURL,
		TrustedProxies: cfg.TrustedProxyCIDRs,
		Static:         static,
		Auth:           mods.auth,
		Settings:       mods.settings,
		Attributes:     mods.attributes,
		Services:       mods.services,
		Clients:        mods.clients,
		Status:         status.New(version),
		Log:            log,
	})
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("serve: %w", err)
	}
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		// net/http's own errors (a bad TLS handshake, a broken connection) go to the JSON log too.
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}, pool, nil
}

// modules are the domain modules serve and the admin subcommands build.
type modules struct {
	settings   *settings.Module
	attributes *attributes.Module
	services   *services.Module
	clients    *clients.Module
	auth       *auth.Auth
}

// newModules builds the modules, for serve and the admin subcommands.
func newModules(cfg config.Config) (modules, error) {
	settingsModule := settings.New(settings.Options{Clock: clock.Real{}, MediaDir: cfg.MediaDir})
	authModule, err := auth.New(auth.Options{
		Clock:           clock.Real{},
		SettingsCreator: settingsCreator(settingsModule),
	})
	if err != nil {
		return modules{}, fmt.Errorf("build the modules: %w", err)
	}
	attributesModule := attributes.New(attributes.Options{Clock: clock.Real{}})
	return modules{
		settings:   settingsModule,
		attributes: attributesModule,
		services:   services.New(services.Options{Clock: clock.Real{}}),
		clients:    clients.New(clients.Options{Clock: clock.Real{}, Attributes: attributesModule}),
		auth:       authModule,
	}, nil
}

// printSetupLink is serve step 4 (design doc item 18): while no user exists it makes a new setup
// token and prints "Setup: <PUBLIC_URL>/setup#<token>" to stdout. The line holds a secret, so it is
// one plain fmt line, not a slog record: the "never log a token" rule has this one exception, by
// design ("Auth, sessions and handover"; OP11).
func printSetupLink(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, authModule *auth.Auth, stdout io.Writer) error {
	needed, err := authModule.SetupNeeded(ctx, pool)
	if err != nil {
		return fmt.Errorf("setup link: %w", err)
	}
	if !needed {
		return nil
	}
	link, err := newSetupLink(ctx, cfg.PublicURL, pool, authModule)
	if err != nil {
		return err
	}
	printf(stdout, "Setup: %s\n", link)
	return nil
}

// settingsCreator is the setup settings creator auth gets (design doc item 40): it converts
// auth.SetupSettings to settings.SettingsInput and calls Create, dropping the returned Settings.
func settingsCreator(m *settings.Module) auth.SettingsCreator {
	return func(ctx context.Context, q db.DBTX, s auth.SetupSettings) error {
		if _, err := m.Create(ctx, q, settings.SettingsInput{
			Name: s.Name, Timezone: s.Timezone, Locale: s.Locale, Currency: s.Currency,
			CountryCode: s.CountryCode, AccentColor: s.AccentColor, PushHour: s.PushHour,
		}); err != nil {
			return fmt.Errorf("create settings: %w", err)
		}
		return nil
	}
}

// closeListener closes a listener serveOn will not serve on. A close error leaves nothing to do
// but say so: serve is already exiting 1.
func closeListener(ln net.Listener) {
	if err := ln.Close(); err != nil {
		slog.Warn("closing the listener failed", "code", "listener_close_failed")
	}
}
