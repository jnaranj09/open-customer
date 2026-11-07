package config

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// placeholderValue is how deploy/.env.example writes a secret the deployer must replace
// ("change-me"), with the spellings a new line might use.
var placeholderValue = regexp.MustCompile(`(?i)^(change|replace|set)[-_]?me$`)

// envLine is one KEY=value line of .env.example, commented out or not.
var envLine = regexp.MustCompile(`^(?:#[ \t]*)?([A-Z][A-Z0-9_]*)=(.*)$`)

// composeRef is a ${KEY...} reference in a compose environment value.
var composeRef = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)`)

// notOurs are the compose services that receive a placeholder variable but are not this repo's
// code to refuse it, with the reason.
var notOurs = map[string]string{
	"db":          "postgres takes any password; app and backup refuse the same DB_PASSWORD",
	"cloudflared": "the tunnel refuses a token that is not a real one by itself",
	"restore":     "a restore must still read a repository made with the placeholder",
}

// placeholderGaps returns every way the three copies of the placeholder rule disagree: a
// placeholder value in .env.example that PlaceholderSecrets lacks, backup.sh's list differing from
// PlaceholderSecrets, and a variable holding a placeholder whose consumer in compose does not
// refuse it (app: Load; backup: a refuse_placeholder line in backup.sh). A consumer outside
// notOurs, app and backup is a gap too: someone must decide who refuses it.
func placeholderGaps(example, compose, script string) []string {
	var gaps []string
	keys := map[string]string{}
	for line := range strings.Lines(example) {
		m := envLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || !placeholderValue.MatchString(m[2]) && !slices.Contains(PlaceholderSecrets, m[2]) {
			continue
		}
		keys[m[1]] = m[2]
		if !slices.Contains(PlaceholderSecrets, m[2]) {
			gaps = append(gaps, fmt.Sprintf(".env.example %s=%s: PlaceholderSecrets lacks the value", m[1], m[2]))
		}
	}
	if len(keys) == 0 {
		gaps = append(gaps, ".env.example: no placeholder secret found")
	}
	list := regexp.MustCompile(`(?m)^PLACEHOLDER_SECRETS=\(([^)]*)\)$`).FindStringSubmatch(script)
	if list == nil {
		gaps = append(gaps, "backup.sh: no PLACEHOLDER_SECRETS=(...) line")
	} else if !slices.Equal(slices.Sorted(slices.Values(strings.Fields(list[1]))), slices.Sorted(slices.Values(PlaceholderSecrets))) {
		gaps = append(gaps, fmt.Sprintf("backup.sh PLACEHOLDER_SECRETS = %q, PlaceholderSecrets = %q", strings.Fields(list[1]), PlaceholderSecrets))
	}
	refused := regexp.MustCompile(`(?m)^[ \t]*refuse_placeholder[ \t]+(.*)$`).FindAllStringSubmatch(script, -1)
	var doc struct {
		Services map[string]struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(compose), &doc); err != nil {
		return append(gaps, fmt.Sprintf("compose.yaml: not valid YAML: %v", err))
	}
	for svc, s := range doc.Services {
		for name, value := range s.Environment {
			ref := composeRef.FindStringSubmatch(value)
			if ref == nil {
				continue
			}
			placeholder, ok := keys[ref[1]]
			if !ok {
				continue
			}
			switch {
			case notOurs[svc] != "":
			case svc == "app":
				m := map[string]string{"PUBLIC_URL": "https://citas.example.com", "DB_PASSWORD": "s3cr3t:/@pass", "VAPID_SUBJECT": "mailto:owner@example.com"}
				m[name] = placeholder
				if _, err := Load(func(k string) string { return m[k] }); err == nil || !strings.Contains(err.Error(), name) {
					gaps = append(gaps, fmt.Sprintf("app: Load does not refuse %s=%s", name, placeholder))
				}
			case svc == "backup":
				if !slices.ContainsFunc(refused, func(r []string) bool { return slices.Contains(strings.Fields(r[1]), name) }) {
					gaps = append(gaps, fmt.Sprintf("backup: backup.sh does not refuse_placeholder %s", name))
				}
			default:
				gaps = append(gaps, fmt.Sprintf("compose service %s gets %s, which holds a placeholder: nothing refuses it", svc, name))
			}
		}
	}
	slices.Sort(gaps)
	return gaps
}

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	b, err := fs.ReadFile(os.DirFS("../../.."), rel)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// Every secret deploy/.env.example ships with a placeholder is refused by the code that reads it
// (threat model T63), and the placeholder lists of Go and backup.sh stay equal.
func TestPlaceholderSecretsMatchEnvExample(t *testing.T) {
	t.Parallel()
	if gaps := placeholderGaps(readRepo(t, "deploy/.env.example"), readRepo(t, "deploy/compose.yaml"), readRepo(t, "deploy/backup/backup.sh")); len(gaps) != 0 {
		t.Errorf("placeholder gaps:\n%s", strings.Join(gaps, "\n"))
	}
}

func TestPlaceholderGapsProbes(t *testing.T) {
	t.Parallel()
	example := "DB_PASSWORD=change-me\nRESTIC_PASSWORD=change-me\nPUBLIC_URL=https://citas.example.com\n"
	compose := "services:\n  app:\n    environment:\n      DB_PASSWORD: ${DB_PASSWORD:?x}\n" +
		"  backup:\n    environment:\n      RESTIC_PASSWORD: ${RESTIC_PASSWORD:?x}\n      PGPASSWORD: ${DB_PASSWORD:?x}\n" +
		"  restore:\n    environment:\n      RESTIC_PASSWORD: ${RESTIC_PASSWORD:?x}\n"
	script := "PLACEHOLDER_SECRETS=(change-me)\n    refuse_placeholder RESTIC_PASSWORD PGPASSWORD\n"
	if got := placeholderGaps(example, compose, script); len(got) != 0 {
		t.Fatalf("a matching set reports %q", got)
	}
	bad := []struct{ name, example, compose, script string }{
		{"a new app secret Load ignores", example + "SMTP_PASSWORD=change-me\n",
			strings.Replace(compose, "  backup:\n", "      SMTP_PASSWORD: ${SMTP_PASSWORD:?x}\n  backup:\n", 1), script},
		{"a backup variable not refused", example,
			compose, "PLACEHOLDER_SECRETS=(change-me)\n    refuse_placeholder RESTIC_PASSWORD\n"},
		{"a placeholder spelling the lists lack", example + "API_TOKEN=replace-me\n", compose, script},
		{"backup.sh list drifts", example, compose, "PLACEHOLDER_SECRETS=(change-me other)\n    refuse_placeholder RESTIC_PASSWORD PGPASSWORD\n"},
		{"a new service gets a placeholder", example,
			compose + "  mailer:\n    environment:\n      PASS: ${DB_PASSWORD:?x}\n", script},
		{"no placeholder at all", "PUBLIC_URL=https://citas.example.com\n", compose, script},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := placeholderGaps(c.example, c.compose, c.script); len(got) == 0 {
				t.Error("no gap reported")
			}
		})
	}
}
