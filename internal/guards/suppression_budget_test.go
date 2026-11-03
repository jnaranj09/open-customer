package guards

// suppression-budget (principle P19): the number of //nolint comments is pinned and only falls. A
// fix lowers pinnedNolint in the same commit. Counted from parsed comments, so a string that
// mentions nolint does not count.

import (
	"go/ast"
	"strings"
	"testing"
)

// pinnedNolint is the //nolint total when the strict linters landed (TASKS.md T4). Only lowered.
const pinnedNolint = 0

func nolintCount(f *ast.File) int {
	n := 0
	for _, g := range f.Comments {
		for _, c := range g.List {
			if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(c.Text, "//")), "nolint") {
				n++
			}
		}
	}
	return n
}

func TestNolintCountProbe(t *testing.T) {
	t.Parallel()
	src := "package p\n\nvar s = \"//nolint:errcheck in a string\"\n\nfunc f() {} //nolint:unused // reason\n\n// nolint:gosec // reason\nvar x = 1\n"
	if got := nolintCount(parseGo(t, "probe.go", []byte(src)).file); got != 2 {
		t.Fatalf("nolintCount = %d, want 2 (the string does not count)", got)
	}
}

func TestSuppressionBudget(t *testing.T) {
	t.Parallel()
	files := goFiles(t, true)
	if len(files) == 0 {
		t.Fatal("no Go files found")
	}
	total := 0
	for _, f := range files {
		if n := nolintCount(f.file); n > 0 {
			t.Logf("%s: %d //nolint", f.rel, n)
			total += n
		}
	}
	if total != pinnedNolint {
		t.Fatalf("//nolint total = %d, pinned %d. A new one: fix the code instead. A removed one: lower pinnedNolint.", total, pinnedNolint)
	}
}
