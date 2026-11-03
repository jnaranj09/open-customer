package guards

// forbid-only (principle P3): a Go test skips only under a condition that names a platform or a
// privilege (runtime.GOOS, runtime.GOARCH, os.Getuid, os.Geteuid). Database tests are kept apart by
// the `db` build tag, never by a skip.

import (
	"go/ast"
	"slices"
	"testing"
)

var skipMethods = []string{"Skip", "Skipf", "SkipNow"}

// unconditionalSkips counts t.Skip* calls that are not inside an `if` whose condition names a
// platform or a privilege.
func unconditionalSkips(f *ast.File) int {
	n := 0
	var stack []ast.Node
	ast.Inspect(f, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		stack = append(stack, node)
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !slices.Contains(skipMethods, sel.Sel.Name) {
			return true
		}
		if !slices.ContainsFunc(stack, func(a ast.Node) bool {
			s, ok := a.(*ast.IfStmt)
			return ok && namesPlatform(f, s.Cond)
		}) {
			n++
		}
		return true
	})
	return n
}

func namesPlatform(f *ast.File, cond ast.Expr) bool {
	found := false
	rt, osName := importName(f, "runtime"), importName(f, "os")
	ast.Inspect(cond, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		switch {
		case !ok:
		case rt != "" && id.Name == rt && (sel.Sel.Name == "GOOS" || sel.Sel.Name == "GOARCH"):
			found = true
		case osName != "" && id.Name == osName && (sel.Sel.Name == "Getuid" || sel.Sel.Name == "Geteuid"):
			found = true
		}
		return !found
	})
	return found
}

func TestUnconditionalSkipsProbes(t *testing.T) {
	t.Parallel()
	src := `package p

import (
	"os"
	"runtime"
	"testing"
)

func TestA(t *testing.T) { t.Skip("flaky") }
func TestB(t *testing.T) {
	if testing.Short() {
		t.Skipf("short")
	}
}
func TestC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no unix sockets")
	}
	if os.Geteuid() == 0 {
		t.SkipNow()
	}
}
`
	if got := unconditionalSkips(parseGo(t, "probe_test.go", []byte(src)).file); got != 2 {
		t.Fatalf("unconditionalSkips = %d, want 2 (the bare skip and the testing.Short skip)", got)
	}
}

func TestForbidOnly(t *testing.T) {
	t.Parallel()
	files := goFiles(t, true)
	if len(files) == 0 {
		t.Fatal("no Go files found")
	}
	for _, f := range files {
		if n := unconditionalSkips(f.file); n > 0 {
			t.Errorf("%s: %d skip(s) outside a platform or privilege condition", f.rel, n)
		}
	}
}
