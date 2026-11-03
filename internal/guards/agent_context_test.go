package guards

// agent-context (principles P8, P10, P20): what loads into every agent session stays short, every
// rule file is path-scoped, every rule names its sensor or says "prose only", and every path the
// living docs name exists (doc-paths).

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// docRoots are the top folders whose backticked paths must exist.
var docRoots = []string{"cmd/", "internal/", "api/", "web/src/", "web/tests/", "deploy/", "scripts/", ".claude/"}

// rulePaths returns the `paths:` globs of a rule file's frontmatter, or nil when it has none.
func rulePaths(text string) []string {
	m := regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n`).FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	lines := strings.Split(m[1], "\n")
	at := slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == "paths:" })
	if at == -1 {
		return nil
	}
	item := regexp.MustCompile(`^ {2}- (['"]?)(.+?)['"]?$`)
	var globs []string
	for _, l := range lines[at+1:] {
		g := item.FindStringSubmatch(l)
		if g == nil {
			break
		}
		globs = append(globs, g[2])
	}
	return globs
}

// unlabeledRules returns top-level bullets (with their wrapped lines) that name no sensor.
func unlabeledRules(text string) []string {
	body := regexp.MustCompile(`(?s)\A---\n.*?\n---\n`).ReplaceAllString(text, "")
	var bullets []string
	current := ""
	flush := func() {
		if current != "" {
			bullets = append(bullets, current)
			current = ""
		}
	}
	for _, line := range strings.Split(body, "\n") {
		switch {
		case strings.HasPrefix(line, "- "):
			flush()
			current = line
		case current != "" && regexp.MustCompile(`^ {2,}\S`).MatchString(line):
			current += " " + strings.TrimSpace(line)
		default:
			flush()
		}
	}
	flush()
	label := regexp.MustCompile(`Sensor:|[Pp]rose only`)
	return slices.DeleteFunc(bullets, label.MatchString)
}

// namedPaths returns the backticked repo paths under docRoots, minus globs, placeholders and
// exact paths .gitignore lists.
func namedPaths(text string, ignored map[string]bool) []string {
	var out []string
	for _, m := range regexp.MustCompile("`([^`\\s]+)`").FindAllStringSubmatch(text, -1) {
		p := m[1]
		if !slices.ContainsFunc(docRoots, func(r string) bool { return strings.HasPrefix(p, r) }) {
			continue
		}
		if strings.ContainsAny(p, "*?{<…") || ignored[p] {
			continue
		}
		out = append(out, p)
	}
	return out
}

// readText reads a repo-relative path.
func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := fs.ReadFile(os.DirFS(root(t)), path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestAgentContextHelpers(t *testing.T) {
	t.Parallel()
	if got := rulePaths("---\npaths:\n  - 'internal/**'\n  - \"web/src/**\"\n---\n# x\n"); !slices.Equal(got, []string{"internal/**", "web/src/**"}) {
		t.Errorf("rulePaths = %q", got)
	}
	if got := rulePaths("# no frontmatter\n"); got != nil {
		t.Errorf("rulePaths without frontmatter = %q, want nil", got)
	}
	if got := unlabeledRules("- one rule\n  wrapped. Sensor: lint\n- two rule\n- three. Prose only.\n"); !slices.Equal(got, []string{"- two rule"}) {
		t.Errorf("unlabeledRules = %q", got)
	}
	ignored := map[string]bool{".claude/settings.local.json": true}
	if got := namedPaths("`.claude/rules/a.md` `.claude/rules/*.md` `internal/<module>/x.go` `.claude/settings.local.json` `README.md`", ignored); !slices.Equal(got, []string{".claude/rules/a.md"}) {
		t.Errorf("namedPaths = %q", got)
	}
}

func TestAgentContext(t *testing.T) {
	t.Parallel()
	base := root(t)
	claude := readText(t, "CLAUDE.md")
	rulesDir := filepath.Join(base, ".claude", "rules")
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	rules := map[string]string{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			rules[e.Name()] = readText(t, ".claude/rules/"+e.Name())
		}
	}
	ignored := map[string]bool{}
	for _, l := range strings.Split(readText(t, ".gitignore"), "\n") {
		if l = strings.TrimPrefix(strings.TrimSpace(l), "/"); l != "" && !strings.HasPrefix(l, "#") {
			ignored[l] = true
		}
	}

	t.Run("CLAUDE.md is at most 200 lines", func(t *testing.T) {
		t.Parallel()
		if n := strings.Count(strings.TrimSuffix(claude, "\n"), "\n") + 1; n > 200 {
			t.Errorf("CLAUDE.md has %d lines", n)
		}
	})
	t.Run("rule files exist (a walk that finds none is broken)", func(t *testing.T) {
		t.Parallel()
		if len(rules) == 0 {
			t.Error("no .claude/rules/*.md found")
		}
	})
	t.Run("every rule file has a non-empty paths: list", func(t *testing.T) {
		t.Parallel()
		for name, text := range rules {
			if len(rulePaths(text)) == 0 {
				t.Errorf("%s has no paths: frontmatter", name)
			}
		}
	})
	t.Run("every rule names its sensor or says prose only", func(t *testing.T) {
		t.Parallel()
		for name, text := range rules {
			for _, b := range unlabeledRules(text) {
				t.Errorf("%s: %s", name, b)
			}
		}
	})
	t.Run("every rules/<x>.md CLAUDE.md names exists", func(t *testing.T) {
		t.Parallel()
		named := regexp.MustCompile(`rules/([\w-]+\.md)`).FindAllStringSubmatch(claude, -1)
		if len(named) == 0 {
			t.Fatal("CLAUDE.md names no rule file")
		}
		for _, m := range named {
			if _, ok := rules[m[1]]; !ok {
				t.Errorf("CLAUDE.md names rules/%s, which does not exist", m[1])
			}
		}
	})
	t.Run("every path CLAUDE.md and the rule files name exists (doc-paths)", func(t *testing.T) {
		t.Parallel()
		docs := map[string]string{"CLAUDE.md": claude}
		for name, text := range rules {
			docs[".claude/rules/"+name] = text
		}
		for doc, text := range docs {
			for _, p := range namedPaths(text, ignored) {
				if _, err := os.Stat(filepath.Join(base, p)); err != nil {
					t.Errorf("%s names `%s`, which does not exist", doc, p)
				}
			}
		}
	})
}
