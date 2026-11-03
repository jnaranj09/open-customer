// Package guards holds repo-wide structural tests (TASKS.md T6). Each guard reads the repo, parses Go
// with go/parser (never a regex over raw text), and fails when a rule is broken. Spec: the
// dev-project-bootstrap guards.md. Every guard was seen red once by breaking what it protects.
package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// root is the repo root: this package sits at internal/guards.
func root(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	return abs
}

// skipDirs are never walked: dependencies, build output, VCS data.
var skipDirs = map[string]bool{"node_modules": true, ".git": true, "bin": true, "build": true, ".svelte-kit": true}

type goFile struct {
	rel  string // repo-relative, with "/"
	fset *token.FileSet
	file *ast.File
	src  []byte
}

// goFiles parses every .go file in the repo (tests included when withTests is true).
func goFiles(t *testing.T, withTests bool) []goFile {
	t.Helper()
	repo := os.DirFS(root(t))
	var out []goFile
	err := fs.WalkDir(repo, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || (!withTests && strings.HasSuffix(path, "_test.go")) {
			return nil
		}
		src, err := fs.ReadFile(repo, path)
		if err != nil {
			return err
		}
		out = append(out, parseGo(t, path, src))
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

func parseGo(t *testing.T, rel string, src []byte) goFile {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	return goFile{rel: rel, fset: fset, file: f, src: src}
}

// importName is the local name a file uses for the import path, or "" when it does not import it.
func importName(f *ast.File, path string) string {
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != path {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return path[strings.LastIndex(path, "/")+1:]
	}
	return ""
}

// isPkgCall reports whether call is <local name of pkgPath>.<fn>(...).
func isPkgCall(f *ast.File, n ast.Node, pkgPath, fn string) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	name := importName(f, pkgPath)
	return ok && name != "" && id.Name == name && sel.Sel.Name == fn
}
