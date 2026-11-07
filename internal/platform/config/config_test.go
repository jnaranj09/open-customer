package config

import (
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func minimal() map[string]string {
	return map[string]string{
		"PUBLIC_URL":    "https://citas.example.com",
		"DB_PASSWORD":   "s3cr3t:/@pass",
		"VAPID_SUBJECT": "mailto:owner@example.com",
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	c, err := Load(env(minimal()))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got, want := c.PublicURL.String(), "https://citas.example.com"; got != want {
		t.Errorf("PublicURL = %q, want %q", got, want)
	}
	// Parsed back, so a password with URL characters is shown to survive the round trip.
	u, err := url.Parse(c.DatabaseURL)
	if err != nil {
		t.Fatalf("DatabaseURL does not parse: %v", err)
	}
	pw, _ := u.User.Password()
	if u.User.Username() != "open_customer" || pw != minimal()["DB_PASSWORD"] || u.Host != "db:5432" ||
		u.Path != "/open_customer" || u.Query().Get("sslmode") != "disable" {
		t.Errorf("DatabaseURL parts = user %q, host %q, path %q, query %q; want the defaults",
			u.User.Username(), u.Host, u.Path, u.RawQuery)
	}
	if len(c.TrustedProxyCIDRs) != 3 {
		t.Errorf("TrustedProxyCIDRs = %v, want the 3 private ranges", c.TrustedProxyCIDRs)
	}
	if c.MediaDir != "/data/media" || c.BackupStatusFile != "/backup-status/status.json" {
		t.Errorf("MediaDir, BackupStatusFile = %q, %q; want the defaults", c.MediaDir, c.BackupStatusFile)
	}
	if c.AllowMigrateWithoutBackup {
		t.Error("AllowMigrateWithoutBackup = true, want false by default")
	}
	if c.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", c.LogLevel)
	}
}

func TestLoadListsEveryMissingVariable(t *testing.T) {
	t.Parallel()
	_, err := Load(env(nil))
	if err == nil {
		t.Fatal("Load() with no env succeeded, want an error")
	}
	for _, name := range []string{"PUBLIC_URL", "DB_PASSWORD", "VAPID_SUBJECT"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, key, value string
	}{
		{"http on a real host", "PUBLIC_URL", "http://citas.example.com"},
		{"a URL with a path", "PUBLIC_URL", "https://citas.example.com/app"},
		{"a relative URL", "PUBLIC_URL", "citas.example.com"},
		{"a subject without mailto", "VAPID_SUBJECT", "owner@example.com"},
		{"a port that is not a number", "DB_PORT", "five"},
		{"a bad CIDR", "TRUSTED_PROXY_CIDRS", "10.0.0.0/8,nope"},
		{"a bad bool", "ALLOW_MIGRATE_WITHOUT_BACKUP", "maybe"},
		{"a bad log level", "LOG_LEVEL", "loud"},
		{"the .env.example placeholder password", "DB_PASSWORD", "change-me"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := minimal()
			m[tc.key] = tc.value
			_, err := Load(env(m))
			if err == nil {
				t.Fatalf("Load() with %s=%q succeeded, want an error", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("error %q does not name %s", err, tc.key)
			}
		})
	}
}

func TestLoadNeverQuotesThePassword(t *testing.T) {
	t.Parallel()
	m := minimal()
	m["PUBLIC_URL"] = "http://citas.example.com"
	_, err := Load(env(m))
	if err == nil || strings.Contains(err.Error(), m["DB_PASSWORD"]) {
		t.Fatalf("error = %v; want an error that does not contain the password", err)
	}
}

func TestLoadAcceptsLocalHTTP(t *testing.T) {
	t.Parallel()
	for _, u := range []string{"http://localhost:5173", "http://127.0.0.1:8080"} {
		m := minimal()
		m["PUBLIC_URL"] = u
		c, err := Load(env(m))
		if err != nil {
			t.Fatalf("Load() with PUBLIC_URL=%q error: %v", u, err)
		}
		if c.PublicURL.String() != u {
			t.Errorf("PublicURL = %q, want %q", c.PublicURL, u)
		}
	}
}

// The placeholder list has three copies: PlaceholderSecrets, backup.sh's PLACEHOLDER_SECRETS and
// the secret values in deploy/.env.example. They must stay equal.
func TestPlaceholderSecretsMatchEnvExample(t *testing.T) {
	t.Parallel()
	example, err := os.ReadFile("../../../deploy/.env.example")
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	var fromExample []string
	for line := range strings.Lines(string(example)) {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		secret := name == "DB_PASSWORD" || name == "RESTIC_PASSWORD" || name == "TUNNEL_TOKEN"
		if ok && secret && !slices.Contains(fromExample, value) {
			fromExample = append(fromExample, value)
		}
	}
	script, err := os.ReadFile("../../../deploy/backup/backup.sh")
	if err != nil {
		t.Fatalf("read backup.sh: %v", err)
	}
	m := regexp.MustCompile(`(?m)^PLACEHOLDER_SECRETS=\(([^)]*)\)$`).FindSubmatch(script)
	if m == nil {
		t.Fatal("backup.sh has no PLACEHOLDER_SECRETS=(...) line")
	}
	fromScript := strings.Fields(string(m[1]))
	slices.Sort(fromExample)
	slices.Sort(fromScript)
	want := slices.Sorted(slices.Values(PlaceholderSecrets))
	if len(fromExample) == 0 {
		t.Fatal("no secret values found in .env.example")
	}
	if !slices.Equal(fromExample, want) {
		t.Errorf(".env.example secret values = %q, PlaceholderSecrets = %q", fromExample, want)
	}
	if !slices.Equal(fromScript, want) {
		t.Errorf("backup.sh PLACEHOLDER_SECRETS = %q, PlaceholderSecrets = %q", fromScript, want)
	}
}
