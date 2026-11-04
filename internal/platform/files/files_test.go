package files

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestWriteSyncedReplacesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []byte("new bytes")
	if err := WriteSynced(path, want); err != nil {
		t.Fatalf("WriteSynced: %v", err)
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	got, err := root.ReadFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("file = %q; want %q", got, want)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries; want only the target file", len(entries))
	}
}

func TestWriteSyncedMissingDirFails(t *testing.T) {
	root := t.TempDir()
	if err := WriteSynced(filepath.Join(root, "missing", "a.bin"), []byte("x")); err == nil {
		t.Fatal("WriteSynced into a missing directory returned nil")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("%d entries left behind; want none", len(entries))
	}
}

// fileAsDir makes a regular file where a parent directory should be, so any write below it fails
// with ENOTDIR (also as root, unlike a permission bit).
func fileAsDir(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "locked-dir")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWriteSyncedParentIsFileErrorHasNoPath(t *testing.T) {
	dir := fileAsDir(t)
	err := WriteSynced(filepath.Join(dir, "secret-name.bin"), []byte("x"))
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("err = %v; want ENOTDIR reachable", err)
	}
	if strings.Contains(err.Error(), "locked-dir") || strings.Contains(err.Error(), "secret-name") {
		t.Errorf("error names a path: %v", err)
	}
}

func TestMkdirAllParentIsFileErrorHasNoPath(t *testing.T) {
	dir := fileAsDir(t)
	err := MkdirAll(filepath.Join(dir, "child-name"), 0o750)
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("err = %v; want ENOTDIR", err)
	}
	if strings.Contains(err.Error(), "locked-dir") || strings.Contains(err.Error(), "child-name") {
		t.Errorf("error names a path: %v", err)
	}
}

func TestWriteSyncedTargetIsDirectoryRemovesTemp(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target-name")
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}
	err := WriteSynced(target, []byte("x"))
	if err == nil {
		t.Fatal("WriteSynced onto a directory returned nil")
	}
	if strings.Contains(err.Error(), root) || strings.Contains(err.Error(), "target-name") || strings.Contains(err.Error(), ".tmp-") {
		t.Errorf("error names a path: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "target-name" {
		t.Errorf("entries = %v; want only the target directory (temp file removed)", entries)
	}
}
