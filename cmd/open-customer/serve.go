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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/exporter"
	"github.com/jnaranj09/open-customer/internal/httpapi"
	"github.com/jnaranj09/open-customer/internal/media"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/config"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/push"
	"github.com/jnaranj09/open-customer/internal/reminders"
	"github.com/jnaranj09/open-customer/internal/replies"
	"github.com/jnaranj09/open-customer/internal/scheduler"
	"github.com/jnaranj09/open-customer/internal/services"
	"github.com/jnaranj09/open-customer/internal/settings"
	"github.com/jnaranj09/open-customer/internal/status"
	"github.com/jnaranj09/open-customer/internal/templates"
	"github.com/jnaranj09/open-customer/web"
)

const (
	// shutdownTimeout is how long in-flight requests get after SIGTERM.
	shutdownTimeout = 10 * time.Second
	// readHeaderTimeout bounds a client that never finishes its request headers.
	readHeaderTimeout = 10 * time.Second
	// readTimeout bounds the whole request, headers and body, so a client cannot drip a body
	// forever (threat model T55). The largest body is a 12 MB photo upload (POST /media): 12 MB in
	// 2 minutes needs 100 KB/s, about 0.8 Mbit/s, which a phone's upload on a weak signal still
	// reaches. There is no WriteTimeout: GET /export streams for as long as the ZIP takes.
	readTimeout = 2 * time.Minute
	// idleTimeout closes a keep-alive connection that sends no new request.
	idleTimeout = 2 * time.Minute
	// timerInterval is the daily timer's tick (design doc "Process model").
	timerInterval = time.Minute
)

// serve runs the server until ctx is done. The steps (design doc item 18):
//  1. load the config and open the pool;
//  2. run db.Migrate behind the backup guard;
//  3. ensure the VAPID keys exist;
//  4. print the setup link while no user exists;
//  5. build the modules and the HTTP server;
//  6. start the daily timer (scheduler.Run, one tick a minute);
//  7. serve on LISTEN_PORT (default 8080);
//  8. shut down gracefully on SIGTERM.
//
// The timer stops with ctx, and serve waits for it before it returns.
//
// It returns 0 after a graceful shutdown and 1 on a config, database, migration or listen error,
// which it prints to stderr.
func serve(ctx context.Context, getenv func(string) string, stdout, stderr io.Writer) int {
	cfg, err := config.Load(getenv)
	if err != nil {
		printf(stderr, "%v\n", err)
		return 1
	}
	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.ListenPort))
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

	srv, timer, pool, err := startup(ctx, cfg, log, stdout)
	if err != nil {
		closeListener(ln)
		printf(stderr, "%v\n", err)
		return 1
	}
	defer pool.Close()

	// Step 6. Deferred after pool.Close, so it runs first: the timer is stopped and has returned
	// before the pool closes, and no goroutine outlives serve.
	ticker := time.NewTicker(timerInterval)
	timerCtx, stopTimer := context.WithCancel(ctx)
	timerDone := make(chan struct{})
	go func() {
		defer close(timerDone)
		timer.Run(timerCtx, ticker.C)
	}()
	defer func() {
		stopTimer()
		<-timerDone
		ticker.Stop()
	}()

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

// startup opens the pool, runs the migrations behind the backup guard, ensures the VAPID keys,
// prints the setup link while no user exists and builds the HTTP server and the daily timer. On an
// error the pool is already closed.
func startup(ctx context.Context, cfg config.Config, log *slog.Logger, stdout io.Writer) (*http.Server, *scheduler.Scheduler, *pgxpool.Pool, error) {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("serve: %w", err)
	}
	err = db.Migrate(ctx, pool, db.MigrateOptions{
		AllowWithoutBackup: cfg.AllowMigrateWithoutBackup,
		BackupStatusFile:   cfg.BackupStatusFile,
		Clock:              clock.Real{},
		Logger:             log,
	})
	if err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("serve: %w", err)
	}
	mods, err := newModules(cfg)
	if err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("serve: %w", err)
	}
	// EnsureKeys writes the two keys as two inserts, so it runs in its own write transaction: a
	// half key pair never persists.
	err = db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return mods.push.EnsureKeys(ctx, tx)
	})
	if err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("serve: vapid keys: %w", err)
	}
	if err := printSetupLink(ctx, cfg, pool, mods.auth, stdout); err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("serve: %w", err)
	}
	static, err := fs.Sub(web.Build, "build")
	if err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("serve: embedded web build: %w", err)
	}
	timer := scheduler.New(scheduler.Options{
		Clock: clock.Real{}, Pool: pool, Settings: mods.settings, Reminders: mods.reminders, Push: mods.push,
	})
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
		Media:          mods.media,
		Templates:      mods.templates,
		Reminders:      mods.reminders,
		Replies:        mods.replies,
		Push:           mods.push,
		Appointments:   mods.appointments,
		Exporter:       exporter.New(exporter.Options{Clock: clock.Real{}, Settings: mods.settings, Attributes: mods.attributes}),
		Status: status.New(status.Options{
			Version: version, Clock: clock.Real{}, MediaDir: cfg.MediaDir, BackupStatusFile: cfg.BackupStatusFile,
			Push: mods.push, Scheduler: timer,
		}),
		Clock: clock.Real{},
		Log:   log,
	})
	if err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("serve: %w", err)
	}
	return newHTTPServer(handler, log), timer, pool, nil
}

// newHTTPServer is the app's http.Server around handler.
func newHTTPServer(handler http.Handler, log *slog.Logger) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		// net/http's own errors (a bad TLS handshake, a broken connection) go to the JSON log too.
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
}

// modules are the domain modules serve and the admin subcommands build.
type modules struct {
	settings     *settings.Module
	attributes   *attributes.Module
	services     *services.Module
	clients      *clients.Module
	appointments *appointments.Module
	media        *media.Module
	templates    *templates.Module
	reminders    *reminders.Module
	replies      *replies.Module
	push         *push.Module
	auth         *auth.Auth
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
	servicesModule := services.New(services.Options{Clock: clock.Real{}})
	return modules{
		settings:     settingsModule,
		attributes:   attributesModule,
		services:     servicesModule,
		clients:      clients.New(clients.Options{Clock: clock.Real{}, Attributes: attributesModule}),
		appointments: appointments.New(appointments.Options{Clock: clock.Real{}, Services: servicesModule, Attributes: attributesModule}),
		media:        media.New(media.Options{Clock: clock.Real{}, MediaDir: cfg.MediaDir}),
		templates:    templates.New(templates.Options{Clock: clock.Real{}, Attributes: attributesModule}),
		reminders:    reminders.New(reminders.Options{Clock: clock.Real{}, Attributes: attributesModule}),
		replies:      replies.New(replies.Options{Clock: clock.Real{}}),
		push:         push.New(push.Options{Clock: clock.Real{}, Subject: cfg.VAPIDSubject}),
		auth:         authModule,
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
