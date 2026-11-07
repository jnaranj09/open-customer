package main

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

// A client that drips its body or holds an idle connection must not keep it forever (threat model
// T55); the export streams, so no write timeout cuts it.
func TestHTTPServerTimeouts(t *testing.T) {
	t.Parallel()
	s := newHTTPServer(http.NotFoundHandler(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	got := map[string]time.Duration{
		"ReadHeaderTimeout": s.ReadHeaderTimeout,
		"ReadTimeout":       s.ReadTimeout,
		"IdleTimeout":       s.IdleTimeout,
		"WriteTimeout":      s.WriteTimeout,
	}
	want := map[string]time.Duration{
		"ReadHeaderTimeout": 10 * time.Second,
		"ReadTimeout":       2 * time.Minute,
		"IdleTimeout":       2 * time.Minute,
		"WriteTimeout":      0,
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %v, want %v", name, got[name], w)
		}
	}
}
