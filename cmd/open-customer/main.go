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

const usage = "usage: open-customer serve|version"

func main() {
	// SIGTERM (docker stop) and Ctrl-C cancel ctx; serve then shuts down gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run dispatches the subcommand and returns the exit code: 0 on success, 1 on bad usage or a
// failed command.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		printf(stderr, "%s\n", usage)
		return 1
	}
	switch args[0] {
	case "serve":
		return serve(ctx, getenv, stdout, stderr)
	case "version":
		printf(stdout, "%s\n", version)
		return 0
	default:
		printf(stderr, "unknown command %q\n%s\n", args[0], usage)
		return 1
	}
}

// printf writes to stdout or stderr. It is the binary's one dropped write error: nothing useful can
// be done if the write fails, and the exit code still says what happened.
func printf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}
