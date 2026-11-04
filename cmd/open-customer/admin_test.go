package main

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthcheck(t *testing.T) {
	t.Parallel()
	answering := func(status int) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		t.Cleanup(srv.Close)
		return srv.URL + "/healthz"
	}
	// An address nothing listens on: bind one, then close it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	closedURL := "http://" + ln.Addr().String() + "/healthz"
	if err := ln.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	tests := []struct {
		name     string
		url      string
		wantCode int
	}{
		{name: "a 200 answer exits 0", url: answering(http.StatusOK), wantCode: 0},
		{name: "a 404 answer exits 1 (only 200 is healthy)", url: answering(http.StatusNotFound), wantCode: 1},
		{name: "a 503 answer exits 1", url: answering(http.StatusServiceUnavailable), wantCode: 1},
		{name: "no server listening exits 1", url: closedURL, wantCode: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			if code := healthcheck(t.Context(), tc.url, &stderr); code != tc.wantCode {
				t.Errorf("healthcheck = %d, want %d (stderr %q)", code, tc.wantCode, stderr.String())
			}
		})
	}
}
