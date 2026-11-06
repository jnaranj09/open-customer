package guards

// threat-model (docs/designs/devsecops.md section 1, decision D12): docs/threat-model.md has one
// table under "## Threats", and every row's "Covered by" cell names a check that exists (a Go test,
// a file, a make target, a CI job) or says in words why there is none (accepted, out of scope,
// manual, or a gap a later task closes).

import (
	"fmt"
	"go/ast"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const threatModelPath = "docs/threat-model.md"

var threatColumns = []string{"ID", "Asset", "Threat", "Entry point", "Covered by"}

// refIndex is what the typed references of the "Covered by" cell are checked against.
type refIndex struct {
	tests       map[string]bool            // Test and Fuzz functions declared in _test.go files
	makeTargets map[string]bool            // targets the Makefile defines
	ciJobs      map[string]map[string]bool // workflow file name → its job ids
	fileExists  func(rel string) bool      // a repo-relative path exists
}

// testNames returns the top-level Test* and Fuzz* functions the given test files declare. A name
// that only appears in a comment or a string literal does not count.
func testNames(files []goFile) map[string]bool {
	names := map[string]bool{}
	for _, f := range files {
		if !strings.HasSuffix(f.rel, "_test.go") {
			continue
		}
		for _, d := range f.file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if ok && fn.Recv == nil && (strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Fuzz")) {
				names[fn.Name.Name] = true
			}
		}
	}
	return names
}

// makeTargets returns the explicit targets of a Makefile: rule lines `name [name...]:`, never a
// variable assignment (`:=`, `::=`) or a special target (`.PHONY`).
func makeTargets(makefile string) map[string]bool {
	rule := regexp.MustCompile(`^([A-Za-z0-9_][A-Za-z0-9_.-]*(?:[ \t]+[A-Za-z0-9_][A-Za-z0-9_.-]*)*)[ \t]*:(?:[^:=]|$)`)
	targets := map[string]bool{}
	for _, line := range strings.Split(makefile, "\n") {
		if m := rule.FindStringSubmatch(line); m != nil {
			for _, name := range strings.Fields(m[1]) {
				targets[name] = true
			}
		}
	}
	return targets
}

// workflowJobs returns the job ids of a GitHub Actions workflow: the keys indented two spaces
// under the top-level `jobs:` key.
func workflowJobs(workflow string) map[string]bool {
	key := regexp.MustCompile(`^  ([A-Za-z0-9_-]+):[ \t]*(?:#.*)?$`)
	jobs := map[string]bool{}
	inJobs := false
	for _, line := range strings.Split(workflow, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			inJobs = strings.HasPrefix(line, "jobs:")
			continue
		}
		if m := key.FindStringSubmatch(line); inJobs && m != nil {
			jobs[m[1]] = true
		}
	}
	return jobs
}

type threatRow struct {
	line  int // 1-based line in the document
	cells []string
}

// tableCells splits a markdown table line into its trimmed cells.
func tableCells(line string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	cells := strings.Split(inner, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// threatTable returns the rows of the one table under "## Threats" (header and separator
// excluded), or the problems that stop it being read.
func threatTable(doc string) ([]threatRow, []string) {
	lines := strings.Split(doc, "\n")
	start := slices.Index(lines, "## Threats")
	if start == -1 {
		return nil, []string{`no "## Threats" section`}
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	var tables [][]threatRow
	inTable := false
	for i := start + 1; i < end; i++ {
		isRow := strings.HasPrefix(strings.TrimSpace(lines[i]), "|")
		if isRow && !inTable {
			tables = append(tables, nil)
		}
		if isRow {
			tables[len(tables)-1] = append(tables[len(tables)-1], threatRow{line: i + 1, cells: tableCells(lines[i])})
		}
		inTable = isRow
	}
	if len(tables) != 1 {
		return nil, []string{fmt.Sprintf(`"## Threats" has %d tables, want exactly 1`, len(tables))}
	}
	table := tables[0]
	if !slices.Equal(table[0].cells, threatColumns) {
		return nil, []string{fmt.Sprintf("line %d: header is %q, want %q", table[0].line, table[0].cells, threatColumns)}
	}
	sep := regexp.MustCompile(`^:?-{3,}:?$`)
	if len(table) < 2 || len(table[1].cells) != len(threatColumns) || slices.ContainsFunc(table[1].cells, func(c string) bool { return !sep.MatchString(c) }) {
		return nil, []string{fmt.Sprintf("line %d: the header has no separator row", table[0].line)}
	}
	if len(table) == 2 {
		return nil, []string{"the threats table has no rows"}
	}
	return table[2:], nil
}

// checkReference returns what is wrong with one typed reference, or "" when it is valid.
func checkReference(ref string, idx refIndex) string {
	kind, value, ok := strings.Cut(ref, ":")
	if !ok {
		return fmt.Sprintf("%q has no type (want test:, file:, make:, ci:, accepted:, out of scope:, manual: or gap:)", ref)
	}
	text := strings.TrimSpace(value)
	switch kind {
	case "test", "file", "make", "ci":
		if value == "" || strings.ContainsAny(value, " \t`") {
			return fmt.Sprintf("%q: %s: takes one name, with no space or backtick", ref, kind)
		}
	case "accepted", "out of scope", "manual":
		if text == "" {
			return fmt.Sprintf("%q: %s: needs its reason", ref, kind)
		}
		return ""
	case "gap":
		n, err := strconv.Atoi(strings.TrimPrefix(text, "Task "))
		if !strings.HasPrefix(text, "Task ") || err != nil || n < 2 || n > 10 {
			return fmt.Sprintf("%q: want gap: Task <2-10>", ref)
		}
		return ""
	default:
		return fmt.Sprintf("%q: unknown reference type %q", ref, kind)
	}
	switch kind {
	case "test":
		if !idx.tests[value] {
			return fmt.Sprintf("%q: no _test.go declares func %s", ref, value)
		}
	case "file":
		if path.IsAbs(value) || path.Clean(value) != value || strings.HasPrefix(value, "..") || !idx.fileExists(value) {
			return fmt.Sprintf("%q: no such repo path", ref)
		}
	case "make":
		if !idx.makeTargets[value] {
			return fmt.Sprintf("%q: the Makefile has no target %s", ref, value)
		}
	case "ci":
		workflow, job, _ := strings.Cut(value, "/")
		if !idx.ciJobs[workflow][job] {
			return fmt.Sprintf("%q: no job %q in .github/workflows/%s", ref, job, workflow)
		}
	}
	return ""
}

// checkThreatModel returns every problem in the threat model document.
func checkThreatModel(doc string, idx refIndex) []string {
	rows, problems := threatTable(doc)
	seen := map[string]int{}
	for _, row := range rows {
		if len(row.cells) != len(threatColumns) {
			problems = append(problems, fmt.Sprintf("line %d: %d cells, want %d", row.line, len(row.cells), len(threatColumns)))
			continue
		}
		for i, cell := range row.cells[:4] {
			if cell == "" {
				problems = append(problems, fmt.Sprintf("line %d: empty %s", row.line, threatColumns[i]))
			}
		}
		if id := row.cells[0]; id != "" {
			if first, dup := seen[id]; dup {
				problems = append(problems, fmt.Sprintf("line %d: duplicate ID %s (first on line %d)", row.line, id, first))
			} else {
				seen[id] = row.line
			}
		}
		covered := row.cells[4]
		if covered == "" {
			problems = append(problems, fmt.Sprintf("line %d: empty Covered by", row.line))
			continue
		}
		for _, ref := range strings.Split(covered, "; ") {
			if p := checkReference(strings.TrimSpace(ref), idx); p != "" {
				problems = append(problems, fmt.Sprintf("line %d: %s", row.line, p))
			}
		}
	}
	return problems
}

func TestThreatModelProbes(t *testing.T) {
	t.Parallel()
	src := "package p\n\nimport \"testing\"\n\n" +
		"func TestReal(t *testing.T) {}\n" +
		"func FuzzReal(f *testing.F) {}\n" +
		"type s struct{}\n\nfunc (s) TestMethod(t *testing.T) {}\n" +
		"// func TestInComment(t *testing.T)\n" +
		"var probe = `func TestInString(t *testing.T) {}`\n"
	tests := testNames([]goFile{parseGo(t, "probe_test.go", []byte(src)), parseGo(t, "shipped.go", []byte("package p\n\nfunc TestShipped() {}\n"))})
	if want := map[string]bool{"TestReal": true, "FuzzReal": true}; !maps.Equal(tests, want) {
		t.Errorf("testNames = %v, want %v", tests, want)
	}
	targets := makeTargets("VAR := x\nX ::= y\n.PHONY: test\ntest: web-stub\n\tgo test\nweb: export V := 1\nlint vuln: x\n$(TOOL):\n")
	if want := map[string]bool{"test": true, "web": true, "lint": true, "vuln": true}; !maps.Equal(targets, want) {
		t.Errorf("makeTargets = %v, want %v", targets, want)
	}
	jobs := workflowJobs("name: ci\non:\n  push:\njobs:\n  gate:\n    runs-on: x\n    steps:\n      - run: y\n  # note:\n\n  security: # scans\n    runs-on: x\nenv:\n  other:\n")
	if want := map[string]bool{"gate": true, "security": true}; !maps.Equal(jobs, want) {
		t.Errorf("workflowJobs = %v, want %v", jobs, want)
	}

	idx := refIndex{
		tests:       map[string]bool{"TestReal": true},
		makeTargets: map[string]bool{"vuln": true},
		ciJobs:      map[string]map[string]bool{"ci.yml": {"gate": true}},
		fileExists:  func(rel string) bool { return rel == "deploy/compose.yaml" },
	}
	doc := func(rows ...string) string {
		return "# T\n\n## Threats\n\n| ID | Asset | Threat | Entry point | Covered by |\n| --- | --- | --- | --- | --- |\n" +
			strings.Join(rows, "\n") + "\n\n## GitHub settings\n\n| not | the table |\n| --- | --- |\n"
	}
	good := "| T1 | a | b | c | test:TestReal; file:deploy/compose.yaml; make:vuln; ci:ci.yml/gate; accepted: why; out of scope: the host owner; manual: monthly; gap: Task 3 |"
	if got := checkThreatModel(doc(good), idx); len(got) != 0 {
		t.Errorf("a valid row is refused: %q", got)
	}
	bad := []struct{ name, doc string }{
		{"no Threats section", "# T\n"},
		{"two tables", doc(good, "", "| ID | Asset | Threat | Entry point | Covered by |", "| --- | --- | --- | --- | --- |", strings.Replace(good, "T1", "T2", 1))},
		{"wrong header", strings.Replace(doc(good), "| Covered by |", "| Check |", 1)},
		{"no rows", doc()},
		{"empty Covered by", doc("| T1 | a | b | c |  |")},
		{"empty Threat", doc("| T1 | a |  | c | make:vuln |")},
		{"missing cell", doc("| T1 | a | b | make:vuln |")},
		{"duplicate ID", doc(good, good)},
		{"unknown type allow:", doc("| T1 | a | b | c | allow:CVE-1 |")},
		{"no type", doc("| T1 | a | b | c | TestReal |")},
		{"test no _test.go declares", doc("| T1 | a | b | c | test:TestGone |")},
		{"test only in a string", doc("| T1 | a | b | c | test:TestInString |")},
		{"file that does not exist", doc("| T1 | a | b | c | file:deploy/gone.yaml |")},
		{"file outside the repo", doc("| T1 | a | b | c | file:../deploy/compose.yaml |")},
		{"make target not in the Makefile", doc("| T1 | a | b | c | make:scan |")},
		{"ci job not in the workflow", doc("| T1 | a | b | c | ci:ci.yml/security |")},
		{"ci workflow that does not exist", doc("| T1 | a | b | c | ci:release.yml/gate |")},
		{"accepted without a reason", doc("| T1 | a | b | c | accepted:  |")},
		{"gap without a task", doc("| T1 | a | b | c | gap: later |")},
		{"gap for task 1", doc("| T1 | a | b | c | gap: Task 1 |")},
		{"backticked test", doc("| T1 | a | b | c | test:`TestReal` |")},
	}
	for _, c := range bad {
		if got := checkThreatModel(c.doc, idx); len(got) == 0 {
			t.Errorf("%s: no problem reported", c.name)
		}
	}
}

func TestThreatModel(t *testing.T) {
	t.Parallel()
	base := root(t)
	workflows, err := filepath.Glob(filepath.Join(base, ".github", "workflows", "*.yml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("no workflows found (err %v)", err)
	}
	ciJobs := map[string]map[string]bool{}
	for _, w := range workflows {
		ciJobs[filepath.Base(w)] = workflowJobs(readText(t, ".github/workflows/"+filepath.Base(w)))
	}
	idx := refIndex{
		tests:       testNames(goFiles(t, true)),
		makeTargets: makeTargets(readText(t, "Makefile")),
		ciJobs:      ciJobs,
		fileExists: func(rel string) bool {
			_, err := os.Stat(filepath.Join(base, filepath.FromSlash(rel)))
			return err == nil
		},
	}
	if len(idx.tests) == 0 || len(idx.makeTargets) == 0 {
		t.Fatalf("empty index: %d tests, %d make targets", len(idx.tests), len(idx.makeTargets))
	}
	for _, p := range checkThreatModel(readText(t, threatModelPath), idx) {
		t.Errorf("%s: %s", threatModelPath, p)
	}
}
