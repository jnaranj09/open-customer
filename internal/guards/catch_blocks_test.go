package guards

// catch-blocks (principle P12): no error is dropped in silence. Two shapes are counted in shipped
// code: an empty `if err != nil {}` (always red), and an assignment that sends every result of a
// call to `_` (allowed only where allowedDrops names the file, at the exact count, with a reason).
// errcheck covers most of the rest; this guard pins what it lets through.

import (
	"go/ast"
	"go/token"
	"maps"
	"testing"
)

// allowedDrops: repo-relative file → exact count and why dropping is safe. Only shrinks.
var allowedDrops = map[string]struct {
	count int
	why   string
}{
	"cmd/open-customer/main.go":            {1, "printf writes to stdout/stderr; nothing useful can be done if they fail, and the exit code still says what happened"},
	"internal/platform/logging/logging.go": {1, "crypto/rand.Read never returns an error since Go 1.24"},
}

type dropScan struct{ emptyErrIfs, blankCalls int }

func scanDrops(f *ast.File) dropScan {
	var r dropScan
	ast.Inspect(f, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.IfStmt:
			if isErrNotNil(s.Cond) && len(s.Body.List) == 0 {
				r.emptyErrIfs++
			}
		case *ast.AssignStmt:
			if allBlank(s.Lhs) && len(s.Rhs) == 1 {
				if _, ok := s.Rhs[0].(*ast.CallExpr); ok {
					r.blankCalls++
				}
			}
		}
		return true
	})
	return r
}

func isErrNotNil(e ast.Expr) bool {
	b, ok := e.(*ast.BinaryExpr)
	if !ok || b.Op != token.NEQ {
		return false
	}
	x, xok := b.X.(*ast.Ident)
	y, yok := b.Y.(*ast.Ident)
	return xok && yok && x.Name == "err" && y.Name == "nil"
}

func allBlank(lhs []ast.Expr) bool {
	for _, e := range lhs {
		if id, ok := e.(*ast.Ident); !ok || id.Name != "_" {
			return false
		}
	}
	return len(lhs) > 0
}

func TestScanDropsProbes(t *testing.T) {
	t.Parallel()
	src := `package p

func f() error { return nil }
func g() (int, error) { return 0, nil }

func h() {
	if err := f(); err != nil {
	}
	_ = f()
	_, _ = g()
	n, _ := g()
	_ = n
	if err := f(); err != nil {
		panic(err)
	}
}
`
	got := scanDrops(parseGo(t, "probe.go", []byte(src)).file)
	if want := (dropScan{emptyErrIfs: 1, blankCalls: 2}); got != want {
		t.Fatalf("scanDrops = %+v, want %+v (a blank value, not a call, does not count)", got, want)
	}
}

func TestCatchBlocks(t *testing.T) {
	t.Parallel()
	files := goFiles(t, false)
	if len(files) == 0 {
		t.Fatal("no shipped Go files found")
	}
	actual := map[string]int{}
	for _, f := range files {
		r := scanDrops(f.file)
		if r.emptyErrIfs > 0 {
			t.Errorf("%s: %d empty `if err != nil {}`: handle or log the error with a code=", f.rel, r.emptyErrIfs)
		}
		if r.blankCalls > 0 {
			actual[f.rel] = r.blankCalls
		}
	}
	expected := map[string]int{}
	for f, a := range allowedDrops {
		expected[f] = a.count
	}
	if !maps.Equal(actual, expected) {
		t.Fatalf("calls with every result sent to _ = %v, allowed %v. A new one: handle the error. A removed one: lower allowedDrops.", actual, expected)
	}
}
