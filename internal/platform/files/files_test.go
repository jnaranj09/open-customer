package files

import (
	"bytes"
	"os"
	"path/filepath"
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
