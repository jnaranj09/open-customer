package guards

// security-allow-list (docs/designs/devsecops.md "Blocking policy", decision D4): every entry in
// security/allow.json names a known tool, a finding id (no whitespace, no comma), a one-line reason and an expiry date that
// has not passed and is at most maxAllowDays ahead; no (tool, id) pair appears twice. A new entry
// or a later date needs the owner's agreement in the commit message (prose only).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
)

const (
	allowListPath = "security/allow.json"
	maxAllowDays  = 180
	allowDate     = "2006-01-02"
)

// allowTools are the tools whose findings the allow-list may silence. scripts/security/allow.sh
// keeps its own copy (TOOLS=); TestSecurityAllowList fails when the two differ.
var allowTools = []string{"trivy", "trivy-config", "zizmor", "semgrep", "zap", "nuclei", "gitleaks"}

type allowEntry struct {
	Tool    string `json:"tool"`
	ID      string `json:"id"`
	Reason  string `json:"reason"`
	Expires string `json:"expires"`
}

// checkAllowList returns every problem in the allow-list, judged on the UTC day today.
func checkAllowList(data []byte, today time.Time, fileExists func(rel string) bool) []string {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return []string{"not a JSON array (an empty list is [])"}
	}
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	latest := day.AddDate(0, 0, maxAllowDays)
	var problems []string
	seen := map[[2]string]int{}
	for i, r := range raw {
		var e allowEntry
		dec := json.NewDecoder(bytes.NewReader(r))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			problems = append(problems, fmt.Sprintf("entry %d: not an object of tool, id, reason and expires", i))
			continue
		}
		for _, f := range []struct{ name, value string }{{"tool", e.Tool}, {"id", e.ID}, {"reason", e.Reason}, {"expires", e.Expires}} {
			if strings.TrimSpace(f.value) == "" {
				problems = append(problems, fmt.Sprintf("entry %d: missing %s", i, f.name))
			}
		}
		if e.Tool != "" && !slices.Contains(allowTools, e.Tool) {
			problems = append(problems, fmt.Sprintf("entry %d: unknown tool %q (want one of %s)", i, e.Tool, strings.Join(allowTools, ", ")))
		}
		if e.Expires != "" {
			exp, err := time.Parse(allowDate, e.Expires)
			switch {
			case err != nil || exp.Format(allowDate) != e.Expires:
				problems = append(problems, fmt.Sprintf("entry %d: expires %q is not YYYY-MM-DD", i, e.Expires))
			case exp.Before(day):
				problems = append(problems, fmt.Sprintf("entry %d: expired on %s; fix the finding or ask the owner for a new date", i, e.Expires))
			case exp.After(latest):
				problems = append(problems, fmt.Sprintf("entry %d: expires %s, more than %d days ahead", i, e.Expires, maxAllowDays))
			}
		}
		if strings.ContainsFunc(e.ID, idSeparator) {
			problems = append(problems, fmt.Sprintf("entry %d: id %q has whitespace, a newline or a comma: a wrapper would read it as two ids", i, e.ID))
		}
		if e.Tool == "zap" && e.ID != "" {
			if p := checkZapID(e.ID); p != "" {
				problems = append(problems, fmt.Sprintf("entry %d: zap id %q: %s", i, e.ID, p))
			}
		}
		if e.Tool == "trivy" && e.ID != "" {
			if p := checkTrivyID(e.ID); p != "" {
				problems = append(problems, fmt.Sprintf("entry %d: trivy id %q: %s", i, e.ID, p))
			}
		}
		if e.Tool == "trivy-config" && e.ID != "" {
			if p := checkTrivyConfigID(e.ID, fileExists); p != "" {
				problems = append(problems, fmt.Sprintf("entry %d: trivy-config id %q: %s", i, e.ID, p))
			}
		}
		key := [2]string{e.Tool, e.ID}
		if first, dup := seen[key]; dup && e.Tool != "" && e.ID != "" {
			problems = append(problems, fmt.Sprintf("entry %d: duplicate of entry %d (%s %s)", i, first, e.Tool, e.ID))
		} else if !dup {
			seen[key] = i
		}
	}
	return problems
}

// idSeparator reports a rune no allow-list id may hold, of any tool: allow.sh prints one id per
// line, and dast.sh joins the nuclei ids with commas, so an id with whitespace or a comma would
// silence two findings with one approved entry.
func idSeparator(r rune) bool { return unicode.IsSpace(r) || r == ',' }

// zapID is a ZAP rule number, alone (the whole rule is IGNORE) or with "@<url regex>" (the rule
// stays as zap-rules.tsv sets it and only alerts on URLs the regex matches from the start are out
// of scope: scripts/security/dast.sh writes a `<id>\tOUTOFSCOPE\t<regex>` rules line).
var zapID = regexp.MustCompile(`^([0-9]+)(?:@(\S+))?$`)

// zapScopeProbes are URLs of the scanned app (the shell, a built file, an API route) that no scope
// may match, so a scope cannot be a catch-all. An allowed scope names some other route.
var zapScopeProbes = []string{
	"http://localhost:8080/",
	"http://localhost:8080/_app/x.js",
	"http://localhost:8080/api/v1/zz-scope-probe",
}

// checkZapID returns what is wrong with a zap id, or "". ZAP compiles the scope with Python's re;
// Go's syntax is close enough to catch a typo or a catch-all: the scope must compile, end in $
// and match none of zapScopeProbes. It does not prove the scope names exactly one route.
func checkZapID(id string) string {
	m := zapID.FindStringSubmatch(id)
	switch {
	case m == nil:
		return "want <rule number> or <rule number>@<url regex> (no space, the regex not empty)"
	case m[2] == "":
		return ""
	}
	re, err := regexp.Compile(m[2])
	switch {
	case err != nil:
		return "the url regex does not compile"
	case !strings.HasSuffix(m[2], "$"):
		return "the url regex must end in $"
	}
	for _, probe := range zapScopeProbes {
		if re.MatchString(probe) {
			return "the url regex matches " + probe + ", so it is a catch-all"
		}
	}
	return ""
}

// probeFileExists stands in for the repo in the probes: only the backup Dockerfile exists.
func probeFileExists(rel string) bool { return rel == "deploy/backup/Dockerfile" }

// trivyID is a CVE id, alone (ignored wherever Trivy finds it) or with "@<path>" (ignored only in
// the result whose Target is that path: scripts/security/trivy-ignore.sh writes the ignore entry
// with Trivy's `paths:`). A Go binary's result has the binary's path in the image as its Target
// (checked on Trivy 0.76.1), so "<CVE id>@usr/local/bin/gosu" leaves the same CVE in our own binary red.
var trivyID = regexp.MustCompile(`^([A-Za-z0-9._:-]+)(?:@([^@]+))?$`)

// trivyScope is a clean relative path: segments of letters, digits, dot, underscore and dash.
var trivyScope = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)

// checkTrivyID returns what is wrong with a trivy id, or "". The scope names a file inside an
// image, so it is not looked up in the repo; it must be a clean relative path with no glob (Trivy
// matches `paths:` as a pattern) and no `..` segment.
func checkTrivyID(id string) string {
	m := trivyID.FindStringSubmatch(id)
	switch {
	case m == nil:
		return "want <CVE id> or <CVE id>@<path in the image> (no space, one @)"
	case !strings.Contains(id, "@"):
		return ""
	case !trivyScope.MatchString(m[2]) || slices.Contains(strings.Split(m[2], "/"), ".."):
		return "the path must be a clean relative path (no glob, no .., no leading or doubled slash)"
	}
	return ""
}

// trivyConfigID is a Trivy check id, alone (the check is ignored in every scanned Dockerfile) or
// with "@<repo path>" (ignored in that file only: scripts/security/config-scan.sh writes the
// ignore entry with Trivy's `paths:`, and refuses a path it does not scan).
var trivyConfigID = regexp.MustCompile(`^([A-Za-z0-9._:-]+)(?:@([A-Za-z0-9._/-]+))?$`)

// checkTrivyConfigID returns what is wrong with a trivy-config id, or "". A scope must be a clean
// repo-relative path to a file that exists (no glob: Trivy matches `paths:` as a pattern).
func checkTrivyConfigID(id string, fileExists func(rel string) bool) string {
	m := trivyConfigID.FindStringSubmatch(id)
	switch {
	case m == nil:
		return "want <check id> or <check id>@<repo path of a Dockerfile> (no space, no glob)"
	case !strings.Contains(id, "@"):
		return ""
	case path.IsAbs(m[2]) || path.Clean(m[2]) != m[2] || strings.HasPrefix(m[2], ".."):
		return "the path must be a clean repo-relative path"
	case !fileExists(m[2]):
		return "no such repo path " + m[2]
	}
	return ""
}

func TestSecurityAllowListProbes(t *testing.T) {
	t.Parallel()
	today := time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)
	entry := func(tool, id, reason, expires string) string {
		return fmt.Sprintf(`{"tool": %q, "id": %q, "reason": %q, "expires": %q}`, tool, id, reason, expires)
	}
	good := entry("trivy", "CVE-2026-0001", "no fixed version", "2026-11-06")
	valid := []struct{ name, data string }{
		{"an empty list", `[]`},
		{"an entry expiring today", "[" + good + "]"},
		{"an entry exactly 180 days ahead", "[" + entry("zap", "10038", "r", "2027-05-05") + "]"},
		{"the same id for two tools", "[" + good + "," + entry("trivy-config", "CVE-2026-0001", "r", "2026-12-01") + "]"},
		{"a zap rule scoped to one route", "[" + entry("zap", "40018@.*/api/v1/client-errors$", "r", "2026-12-01") + "]"},
		{"a trivy-config check", "[" + entry("trivy-config", "DS-0002", "r", "2026-12-01") + "]"},
		{"a trivy CVE scoped to one binary in an image", "[" + entry("trivy", "CVE-2026-0001@usr/local/bin/gosu", "r", "2026-12-01") + "]"},
		{"a trivy-config check scoped to one file", "[" + entry("trivy-config", "DS-0002@deploy/backup/Dockerfile", "r", "2026-12-01") + "]"},
	}
	for _, c := range valid {
		if got := checkAllowList([]byte(c.data), today, probeFileExists); len(got) != 0 {
			t.Errorf("%s: refused: %q", c.name, got)
		}
	}
	bad := []struct{ name, data string }{
		{"not JSON", `[`},
		{"null", `null`},
		{"an object, not a list", `{}`},
		{"a string entry", `["x"]`},
		{"an unknown field", `[{"tool": "zap", "id": "1", "reason": "r", "expires": "2026-12-01", "owner": "x"}]`},
		{"missing tool", "[" + entry("", "1", "r", "2026-12-01") + "]"},
		{"missing id", "[" + entry("zap", "", "r", "2026-12-01") + "]"},
		{"missing reason", "[" + entry("zap", "1", " ", "2026-12-01") + "]"},
		{"missing expires", "[" + entry("zap", "1", "r", "") + "]"},
		{"unknown tool", "[" + entry("checkov", "1", "r", "2026-12-01") + "]"},
		{"bad date", "[" + entry("zap", "1", "r", "2026-13-01") + "]"},
		{"date in another form", "[" + entry("zap", "1", "r", "01/12/2026") + "]"},
		{"expired yesterday", "[" + entry("zap", "1", "r", "2026-11-05") + "]"},
		{"181 days ahead", "[" + entry("zap", "1", "r", "2027-05-06") + "]"},
		{"duplicate tool and id", "[" + good + "," + entry("trivy", "CVE-2026-0001", "other", "2026-12-01") + "]"},
		{"a zap id that is not a rule number", "[" + entry("zap", "sql-injection", "r", "2026-12-01") + "]"},
		{"a zap scope that is empty", "[" + entry("zap", "40018@", "r", "2026-12-01") + "]"},
		{"a zap scope with a space", "[" + entry("zap", "40018@.*/a b$", "r", "2026-12-01") + "]"},
		{"a zap scope that does not compile", "[" + entry("zap", "40018@.*/a(b$", "r", "2026-12-01") + "]"},
		{"a zap scope not anchored with $", "[" + entry("zap", "40018@.*/api/v1/client-errors", "r", "2026-12-01") + "]"},
		{"a zap scope that matches every route", "[" + entry("zap", "40018@.*$", "r", "2026-12-01") + "]"},
		{"a zap scope that matches the app shell", "[" + entry("zap", "40018@http://localhost:8080/$", "r", "2026-12-01") + "]"},
		{"a zap scope that matches every built file", "[" + entry("zap", "40018@.*\\.js$", "r", "2026-12-01") + "]"},
		{"a zap scope that matches every API route", "[" + entry("zap", "40018@.*/api/v1/.*$", "r", "2026-12-01") + "]"},
		{"a trivy-config scope that is empty", "[" + entry("trivy-config", "DS-0002@", "r", "2026-12-01") + "]"},
		{"a trivy-config scope with no check", "[" + entry("trivy-config", "@deploy/backup/Dockerfile", "r", "2026-12-01") + "]"},
		{"a trivy-config check with a space", "[" + entry("trivy-config", "DS 0002", "r", "2026-12-01") + "]"},
		{"a trivy-config scope to a missing file", "[" + entry("trivy-config", "DS-0002@deploy/gone/Dockerfile", "r", "2026-12-01") + "]"},
		{"a trivy-config scope with an absolute path", "[" + entry("trivy-config", "DS-0002@/deploy/backup/Dockerfile", "r", "2026-12-01") + "]"},
		{"a trivy-config scope outside the repo", "[" + entry("trivy-config", "DS-0002@../deploy/backup/Dockerfile", "r", "2026-12-01") + "]"},
		{"a trivy-config scope that is not a clean path", "[" + entry("trivy-config", "DS-0002@deploy//backup/Dockerfile", "r", "2026-12-01") + "]"},
		{"a trivy-config scope with a glob", "[" + entry("trivy-config", "DS-0002@deploy/*/Dockerfile", "r", "2026-12-01") + "]"},
		{"a trivy id with a newline", "[" + entry("trivy", "CVE-2026-0001\nCVE-2026-0002", "r", "2026-12-01") + "]"},
		{"a trivy id with a space", "[" + entry("trivy", "CVE-2026-0001 CVE-2026-0002", "r", "2026-12-01") + "]"},
		{"a trivy id with a comma", "[" + entry("trivy", "CVE-2026-0001,CVE-2026-0002", "r", "2026-12-01") + "]"},
		{"a trivy-config id with a newline", "[" + entry("trivy-config", "DS-0001\nDS-0002", "r", "2026-12-01") + "]"},
		{"a trivy-config id with a comma", "[" + entry("trivy-config", "DS-0001,DS-0002", "r", "2026-12-01") + "]"},
		{"a zizmor id with a newline", "[" + entry("zizmor", "artipacked\nexcessive-permissions", "r", "2026-12-01") + "]"},
		{"a zizmor id with a comma", "[" + entry("zizmor", "artipacked,excessive-permissions", "r", "2026-12-01") + "]"},
		{"a semgrep id with a tab", "[" + entry("semgrep", "ts-no-eval\tts-no-inner-html", "r", "2026-12-01") + "]"},
		{"a semgrep id with a comma", "[" + entry("semgrep", "ts-no-eval,ts-no-inner-html", "r", "2026-12-01") + "]"},
		{"a zap id with a newline", "[" + entry("zap", "10021\n10035", "r", "2026-12-01") + "]"},
		{"a zap scope with a comma", "[" + entry("zap", "40018@.*/api/v1/a{1,2}$", "r", "2026-12-01") + "]"},
		{"a nuclei id with a comma", "[" + entry("nuclei", "made-up-a,made-up-b", "r", "2026-12-01") + "]"},
		{"a nuclei id with a newline", "[" + entry("nuclei", "made-up-a\nmade-up-b", "r", "2026-12-01") + "]"},
		{"a gitleaks id with a space", "[" + entry("gitleaks", "made-up a", "r", "2026-12-01") + "]"},
		{"a gitleaks id with a non-breaking space", "[" + entry("gitleaks", "made-up\u00a0a", "r", "2026-12-01") + "]"},
		{"a trivy scope that is empty", "[" + entry("trivy", "CVE-2026-0001@", "r", "2026-12-01") + "]"},
		{"a trivy scope with no CVE id", "[" + entry("trivy", "@usr/local/bin/gosu", "r", "2026-12-01") + "]"},
		{"a trivy scope with an absolute path", "[" + entry("trivy", "CVE-2026-0001@/usr/local/bin/gosu", "r", "2026-12-01") + "]"},
		{"a trivy scope with ..", "[" + entry("trivy", "CVE-2026-0001@usr/../bin/gosu", "r", "2026-12-01") + "]"},
		{"a trivy scope outside the image", "[" + entry("trivy", "CVE-2026-0001@../gosu", "r", "2026-12-01") + "]"},
		{"a trivy scope that is not a clean path", "[" + entry("trivy", "CVE-2026-0001@usr//local/bin/gosu", "r", "2026-12-01") + "]"},
		{"a trivy scope with a trailing slash", "[" + entry("trivy", "CVE-2026-0001@usr/local/bin/", "r", "2026-12-01") + "]"},
		{"a trivy scope with a glob", "[" + entry("trivy", "CVE-2026-0001@usr/local/bin/*", "r", "2026-12-01") + "]"},
		{"a trivy scope with a space", "[" + entry("trivy", "CVE-2026-0001@usr/local/bin/go su", "r", "2026-12-01") + "]"},
		{"a trivy scope with two @", "[" + entry("trivy", "CVE-2026-0001@usr/local/bin/gosu@x", "r", "2026-12-01") + "]"},
		{"a trivy-config scope with two @", "[" + entry("trivy-config", "DS-0002@deploy/backup/Dockerfile@x", "r", "2026-12-01") + "]"},
	}
	for _, c := range bad {
		if got := checkAllowList([]byte(c.data), today, probeFileExists); len(got) == 0 {
			t.Errorf("%s: no problem reported", c.name)
		}
	}
	if got := scriptTools("#!/bin/sh\nTOOLS=\"zap  nuclei\"\n# TOOLS=\"x\"\n"); !slices.Equal(got, []string{"zap", "nuclei"}) {
		t.Errorf("scriptTools = %q", got)
	}
}

// scriptTools returns the names in allow.sh's `TOOLS="..."` line, or nil when it has none.
func scriptTools(script string) []string {
	m := regexp.MustCompile(`(?m)^TOOLS="([^"]*)"$`).FindStringSubmatch(script)
	if m == nil {
		return nil
	}
	return strings.Fields(m[1])
}

func TestSecurityAllowList(t *testing.T) {
	t.Parallel()
	if got := scriptTools(readText(t, "scripts/security/allow.sh")); !slices.Equal(got, allowTools) {
		t.Errorf("scripts/security/allow.sh TOOLS = %q, want the guard's %q", got, allowTools)
	}
	base := root(t)
	fileExists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(base, filepath.FromSlash(rel)))
		return err == nil
	}
	for _, p := range checkAllowList([]byte(readText(t, allowListPath)), time.Now().UTC(), fileExists) {
		t.Errorf("%s: %s", allowListPath, p)
	}
}
