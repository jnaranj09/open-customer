package main

import (
	"bytes"
	"strings"
	"testing"
)

// fullUsage is the usage line, spelled out so a wrong one fails here.
const fullUsage = "usage: open-customer serve|reset-password <username>|setup-link|healthcheck|version"

func TestRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		wantCode   int
		wantStdout string
		wantStderr []string
	}{
		{name: "version prints the build version", args: []string{"version"}, wantCode: 0, wantStdout: "dev\n"},
		{name: "no subcommand prints usage", args: nil, wantCode: 1, wantStderr: []string{fullUsage}},
		{name: "unknown subcommand is named", args: []string{"bogus"}, wantCode: 1, wantStderr: []string{`unknown command "bogus"`, "usage:"}},
		{name: "serve with an extra argument prints usage", args: []string{"serve", "extra"}, wantCode: 1, wantStderr: []string{"usage:"}},
		{name: "reset-password without a username prints usage", args: []string{"reset-password"}, wantCode: 1, wantStderr: []string{fullUsage}},
		{name: "reset-password with two usernames prints usage", args: []string{"reset-password", "a", "b"}, wantCode: 1, wantStderr: []string{fullUsage}},
		{name: "setup-link with an extra argument prints usage", args: []string{"setup-link", "extra"}, wantCode: 1, wantStderr: []string{fullUsage}},
		{name: "healthcheck with an extra argument prints usage", args: []string{"healthcheck", "extra"}, wantCode: 1, wantStderr: []string{fullUsage}},
		{name: "version with an extra argument prints usage", args: []string{"version", "extra"}, wantCode: 1, wantStderr: []string{fullUsage}},
		{
			name: "serve with the required variables missing names each one", args: []string{"serve"}, wantCode: 1,
			wantStderr: []string{"PUBLIC_URL", "DB_PASSWORD", "VAPID_SUBJECT"},
		},
		{
			name: "serve with a plain-http PUBLIC_URL on a real host exits 1", args: []string{"serve"}, wantCode: 1,
			env: map[string]string{
				"PUBLIC_URL": "http://example.test", "DB_PASSWORD": "pw", "VAPID_SUBJECT": "mailto:dev@example.test",
			},
			wantStderr: []string{"invalid: PUBLIC_URL"},
		},
		{
			name: "reset-password with the required variables missing exits 1", args: []string{"reset-password", "ana"}, wantCode: 1,
			wantStderr: []string{"PUBLIC_URL", "DB_PASSWORD", "VAPID_SUBJECT"},
		},
		{
			name: "setup-link with the required variables missing exits 1", args: []string{"setup-link"}, wantCode: 1,
			wantStderr: []string{"PUBLIC_URL", "DB_PASSWORD", "VAPID_SUBJECT"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			getenv := func(k string) string { return tc.env[k] }
			code := run(t.Context(), tc.args, getenv, &stdout, &stderr)
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d", code, tc.wantCode)
			}
			if stdout.String() != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tc.wantStdout)
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want)
				}
			}
		})
	}
}
