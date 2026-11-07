package guards

// security-allow-list (docs/designs/devsecops.md "Blocking policy", decision D4): every entry in
// security/allow.json names a known tool, a finding id, a one-line reason and an expiry date that
// has not passed and is at most maxAllowDays ahead; no (tool, id) pair appears twice. A new entry
// or a later date needs the owner's agreement in the commit message (prose only).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
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
func checkAllowList(data []byte, today time.Time) []string {
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
		if e.Tool == "zap" && e.ID != "" {
			if p := checkZapID(e.ID); p != "" {
				problems = append(problems, fmt.Sprintf("entry %d: zap id %q: %s", i, e.ID, p))
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

// zapID is a ZAP rule number, alone (the whole rule is IGNORE) or with "@<url regex>" (the rule
// stays as zap-rules.tsv sets it and only alerts on URLs the regex matches from the start are out
// of scope: scripts/security/dast.sh writes a `<id>\tOUTOFSCOPE\t<regex>` rules line).
var zapID = regexp.MustCompile(`^([0-9]+)(?:@(\S+))?$`)

// zapScopeProbe is a route no scope may match, so a scope cannot cover every route.
const zapScopeProbe = "http://localhost:8080/api/v1/zz-scope-probe"

// checkZapID returns what is wrong with a zap id, or "". ZAP compiles the scope with Python's re;
// Go's syntax is close enough to catch a typo, and the scope must end in $ so it names one route.
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
		return "the url regex must end in $, so it names one route"
	case re.MatchString(zapScopeProbe):
		return "the url regex matches every route"
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
	}
	for _, c := range valid {
		if got := checkAllowList([]byte(c.data), today); len(got) != 0 {
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
	}
	for _, c := range bad {
		if got := checkAllowList([]byte(c.data), today); len(got) == 0 {
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
	for _, p := range checkAllowList([]byte(readText(t, allowListPath)), time.Now().UTC()) {
		t.Errorf("%s: %s", allowListPath, p)
	}
}
