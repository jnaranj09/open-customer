package guards

// semgrep-verbs (docs/designs/devsecops.md section 2; threat T96): the Semgrep rule
// non-get-handler-in-withtx finds a non-GET handler by the verb its name starts with, because
// Semgrep cannot see the HTTP method. This guard keeps that name regex true to api/openapi.yaml:
// every operationId of a non-GET operation must match it, and no GET operationId may. The regex is
// read from the rule file, so there is one copy.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const writeVerbRulePath = "security/semgrep/non-get-handler-in-withtx.yaml"

// ruleHandlerRegex returns the regex of the rule's metavariable-regex on $H.
func ruleHandlerRegex(ruleYAML []byte) (*regexp.Regexp, error) {
	var doc struct {
		Rules []struct {
			Patterns []struct {
				MetavariableRegex *struct {
					Metavariable string `yaml:"metavariable"`
					Regex        string `yaml:"regex"`
				} `yaml:"metavariable-regex"`
			} `yaml:"patterns"`
		} `yaml:"rules"`
	}
	if err := yaml.Unmarshal(ruleYAML, &doc); err != nil {
		return nil, fmt.Errorf("parse the rule: %w", err)
	}
	for _, r := range doc.Rules {
		for _, p := range r.Patterns {
			if p.MetavariableRegex != nil && p.MetavariableRegex.Metavariable == "$H" {
				re, err := regexp.Compile(p.MetavariableRegex.Regex)
				if err != nil {
					return nil, fmt.Errorf("compile the handler regex: %w", err)
				}
				return re, nil
			}
		}
	}
	return nil, fmt.Errorf("the rule has no metavariable-regex on $H")
}

// checkWriteVerbs returns every operationId the rule's regex judges wrongly.
func checkWriteVerbs(openapi, ruleYAML []byte) []string {
	re, err := ruleHandlerRegex(ruleYAML)
	if err != nil {
		return []string{err.Error()}
	}
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `yaml:"operationId"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(openapi, &spec); err != nil {
		return []string{fmt.Sprintf("parse the OpenAPI spec: %v", err)}
	}
	methods := []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
	var problems []string
	seen := 0
	for path, item := range spec.Paths {
		for method, op := range item {
			if !slices.Contains(methods, method) {
				continue
			}
			if op.OperationID == "" {
				problems = append(problems, fmt.Sprintf("%s %s has no operationId", strings.ToUpper(method), path))
				continue
			}
			seen++
			handler := strings.ToUpper(op.OperationID[:1]) + op.OperationID[1:]
			matched := re.MatchString(handler)
			switch {
			case method != "get" && !matched:
				problems = append(problems, fmt.Sprintf("%s %s: handler %s is not matched by the rule's verb regex: add its verb to %s and its fixture", strings.ToUpper(method), path, handler, writeVerbRulePath))
			case method == "get" && matched:
				problems = append(problems, fmt.Sprintf("GET %s: handler %s is matched by the rule's verb regex, so it would be flagged for using s.pool", path, handler))
			}
		}
	}
	if seen == 0 {
		problems = append(problems, "the OpenAPI spec lists no operation")
	}
	slices.Sort(problems)
	return problems
}

func TestSemgrepWriteVerbsMatchTheSpec(t *testing.T) {
	t.Parallel()
	spec, err := os.ReadFile(filepath.Join(root(t), "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read the spec: %v", err)
	}
	rule, err := os.ReadFile(filepath.Join(root(t), writeVerbRulePath))
	if err != nil {
		t.Fatalf("read the rule: %v", err)
	}
	for _, p := range checkWriteVerbs(spec, rule) {
		t.Error(p)
	}
}

func TestSemgrepWriteVerbsProbes(t *testing.T) {
	t.Parallel()
	rule := []byte("rules:\n  - id: x\n    patterns:\n      - pattern: s.pool\n      - metavariable-regex:\n          metavariable: $H\n          regex: ^(Create|Update)\n")
	spec := func(ops string) []byte { return []byte("paths:\n  /a:\n" + ops) }
	cases := []struct {
		name string
		spec []byte
		rule []byte
		want string
	}{
		{"matching verbs pass", spec("    get:\n      operationId: getA\n    post:\n      operationId: createA\n"), rule, ""},
		{"a non-GET with a new verb", spec("    post:\n      operationId: createA\n    delete:\n      operationId: removeA\n"), rule, "handler RemoveA is not matched"},
		{"a GET the regex matches", spec("    get:\n      operationId: updateA\n"), rule, "handler UpdateA is matched"},
		{"an operation with no id", spec("    post:\n      summary: x\n"), rule, "has no operationId"},
		{"a rule with no regex", spec("    get:\n      operationId: getA\n"), []byte("rules:\n  - id: x\n"), "no metavariable-regex on $H"},
		{"a spec with no operation", []byte("paths: {}\n"), rule, "lists no operation"},
	}
	for _, c := range cases {
		got := strings.Join(checkWriteVerbs(c.spec, c.rule), "\n")
		if c.want == "" && got != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s: problems %q, want one containing %q", c.name, got, c.want)
		}
	}
}
