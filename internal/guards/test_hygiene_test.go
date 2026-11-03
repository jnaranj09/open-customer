package guards

// test-hygiene (principle P17): tests use t.TempDir (never os.MkdirTemp), inject time (never
// time.Sleep), and stay under maxTestLines.

import (
	"bytes"
	"go/ast"
	"strings"
	"testing"
)

const maxTestLines = 1000

type hygiene struct{ mkdirTemp, sleep int }

func scanHygiene(f *ast.File) hygiene {
	var r hygiene
	ast.Inspect(f, func(n ast.Node) bool {
		if isPkgCall(f, n, "os", "MkdirTemp") {
			r.mkdirTemp++
		}
		if isPkgCall(f, n, "time", "Sleep") {
			r.sleep++
		}
		return true
	})
	return r
}

func TestScanHygieneProbes(t *testing.T) {
	t.Parallel()
	src := `package p

import (
	"os"
	tm "time"
)

// time.Sleep in a comment does not count.
func f() {
	_, _ = os.MkdirTemp("", "x")
	tm.Sleep(1)
	_ = "os.MkdirTemp"
}
`
	got := scanHygiene(parseGo(t, "probe_test.go", []byte(src)).file)
	if want := (hygiene{mkdirTemp: 1, sleep: 1}); got != want {
		t.Fatalf("scanHygiene = %+v, want %+v", got, want)
	}
}

func TestTestHygiene(t *testing.T) {
	t.Parallel()
	var tests []goFile
	for _, f := range goFiles(t, true) {
		if strings.HasSuffix(f.rel, "_test.go") {
			tests = append(tests, f)
		}
	}
	if len(tests) == 0 {
		t.Fatal("no Go test files found")
	}
	for _, f := range tests {
		r := scanHygiene(f.file)
		if r.mkdirTemp > 0 {
			t.Errorf("%s: os.MkdirTemp; use t.TempDir()", f.rel)
		}
		if r.sleep > 0 {
			t.Errorf("%s: time.Sleep; use the fake clock (internal/platform/clock)", f.rel)
		}
		if n := bytes.Count(bytes.TrimSuffix(f.src, []byte("\n")), []byte("\n")) + 1; n > maxTestLines {
			t.Errorf("%s: %d lines (max %d); split it along its groups", f.rel, n, maxTestLines)
		}
	}
}
