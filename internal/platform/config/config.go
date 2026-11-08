// Package config reads and checks the app's own .env variables (the "app" rows of the design doc's
// Configuration table). It is the one parse site for the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Config is the parsed app configuration.
type Config struct {
	PublicURL                 *url.URL // never nil when Load returns no error
	DatabaseURL               string
	VAPIDSubject              string
	TrustedProxyCIDRs         []netip.Prefix
	MediaDir                  string
	BackupStatusFile          string
	AllowMigrateWithoutBackup bool
	LogLevel                  slog.Level
	ListenPort                int // 1-65535; DefaultListenPort when LISTEN_PORT is empty
}

// PlaceholderSecrets are the secret values deploy/.env.example publishes. A secret that still holds
// one is refused: the value is public in this repo (threat model T63). deploy/backup/backup.sh keeps
// the same list for RESTIC_PASSWORD and the database password. TestPlaceholderSecretsMatchEnvExample
// keeps the two lists and .env.example equal, and checks that each placeholder variable's consumer
// in deploy/compose.yaml refuses it.
var PlaceholderSecrets = []string{"change-me"}

// DefaultListenPort is the port `serve` listens on, and `healthcheck` probes, when LISTEN_PORT is empty.
const DefaultListenPort = 8080

const (
	defaultTrustedProxyCIDRs = "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
	defaultMediaDir          = "/data/media"
	defaultBackupStatusFile  = "/backup-status/status.json"
)

// Load reads the variables through getenv (os.Getenv in the app). It returns one error that names
// every missing or bad variable, and never quotes a value (DB_PASSWORD must not reach a log).
func Load(getenv func(string) string) (Config, error) {
	var missing, bad []string
	get := func(name, def string) string {
		if v := strings.TrimSpace(getenv(name)); v != "" {
			return v
		}
		return def
	}
	required := func(name string) string {
		v := get(name, "")
		if v == "" {
			missing = append(missing, name)
		}
		return v
	}

	var c Config

	if raw := required("PUBLIC_URL"); raw != "" {
		u, err := parsePublicURL(raw)
		if err != nil {
			bad = append(bad, "PUBLIC_URL ("+err.Error()+")")
		}
		c.PublicURL = u
	}

	password := required("DB_PASSWORD")
	// Only DB_PASSWORD is checked here: it is the one placeholder secret the app receives. Compose
	// gives RESTIC_PASSWORD only to the backup and restore services (backup.sh refuses it) and
	// TUNNEL_TOKEN only to cloudflared.
	if slices.Contains(PlaceholderSecrets, password) {
		bad = append(bad, "DB_PASSWORD (still the placeholder from .env.example: set a real password)")
	}
	c.VAPIDSubject = required("VAPID_SUBJECT")
	if c.VAPIDSubject != "" && !strings.HasPrefix(c.VAPIDSubject, "mailto:") {
		bad = append(bad, "VAPID_SUBJECT (must start with mailto:)")
	}

	c.ListenPort = DefaultListenPort
	if raw := get("LISTEN_PORT", ""); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 65535 {
			bad = append(bad, "LISTEN_PORT (must be a port number)")
		} else {
			c.ListenPort = n
		}
	}

	port := get("DB_PORT", "5432")
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		bad = append(bad, "DB_PORT (must be a port number)")
	}
	c.DatabaseURL = (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(get("DB_USER", "open_customer"), password),
		Host:     get("DB_HOST", "db") + ":" + port,
		Path:     "/" + get("DB_NAME", "open_customer"),
		RawQuery: url.Values{"sslmode": {get("DB_SSLMODE", "disable")}}.Encode(),
	}).String()

	for _, s := range strings.Split(get("TRUSTED_PROXY_CIDRS", defaultTrustedProxyCIDRs), ",") {
		p, err := netip.ParsePrefix(strings.TrimSpace(s))
		if err != nil {
			bad = append(bad, "TRUSTED_PROXY_CIDRS (must be a comma-separated list of CIDRs)")
			break
		}
		c.TrustedProxyCIDRs = append(c.TrustedProxyCIDRs, p)
	}

	c.MediaDir = get("MEDIA_DIR", defaultMediaDir)
	c.BackupStatusFile = get("BACKUP_STATUS_FILE", defaultBackupStatusFile)

	allow, err := strconv.ParseBool(get("ALLOW_MIGRATE_WITHOUT_BACKUP", "false"))
	if err != nil {
		bad = append(bad, "ALLOW_MIGRATE_WITHOUT_BACKUP (must be true or false)")
	}
	c.AllowMigrateWithoutBackup = allow

	if err := c.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		bad = append(bad, "LOG_LEVEL (must be debug, info, warn or error)")
	}

	if len(missing) > 0 || len(bad) > 0 {
		return Config{}, configError(missing, bad)
	}
	return c, nil
}

// parsePublicURL requires https, except for a localhost or 127.0.0.1 host (dev and e2e). There is
// no "insecure" flag, so nothing can turn off https on a real subdomain.
func parsePublicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, errors.New("must be an absolute URL")
	}
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, errors.New("must be only scheme, host and port")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && local:
	default:
		return nil, errors.New("must be https unless the host is localhost or 127.0.0.1")
	}
	u.Path = ""
	return u, nil
}

func configError(missing, bad []string) error {
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "missing: "+strings.Join(missing, ", "))
	}
	if len(bad) > 0 {
		parts = append(parts, "invalid: "+strings.Join(bad, ", "))
	}
	return fmt.Errorf("config: %s", strings.Join(parts, "; "))
}
