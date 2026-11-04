// Command open-customer is the app's single binary. Each build phase adds its own subcommands.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	// The distroless image has no zoneinfo; this embeds time zone data so LoadLocation works.
	_ "time/tzdata"
)

// version is set at build time with -ldflags "-X main.version=<version>".
var version = "dev"

const usage = "usage: open-customer serve|reset-password <username>|setup-link|healthcheck|version"

func main() {
	// SIGTERM (docker stop) and Ctrl-C cancel ctx; serve then shuts down gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// argCounts is each subcommand's argument count, the subcommand included.
var argCounts = map[string]int{"serve": 1, "reset-password": 2, "setup-link": 1, "healthcheck": 1, "version": 1}

// run dispatches the subcommand and returns the exit code: 0 on success, 1 on bad usage or a
// failed command. The arguments are outside input: an unknown subcommand or a wrong argument count
// prints the usage and exits 1.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printf(stderr, "%s\n", usage)
		return 1
	}
	want, known := argCounts[args[0]]
	if !known {
		printf(stderr, "unknown command %q\n%s\n", args[0], usage)
		return 1
	}
	if len(args) != want {
		printf(stderr, "%s\n", usage)
		return 1
	}
	switch args[0] {
	case "serve":
		return serve(ctx, getenv, stdout, stderr)
	case "reset-password":
		return resetPassword(ctx, getenv, args[1], stdout, stderr)
	case "setup-link":
		return setupLink(ctx, getenv, stdout, stderr)
	case "healthcheck":
		return healthcheck(ctx, healthzURL, stderr)
	default: // "version": argCounts lets no other name through
		printf(stdout, "%s\n", version)
		return 0
	}
}

// printf writes to stdout or stderr. It is the binary's one dropped write error: nothing useful can
// be done if the write fails, and the exit code still says what happened.
func printf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}
