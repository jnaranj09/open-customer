// Package files holds the durable file write shared by settings icons and media photos. No error
// it returns contains a path, so callers can log or wrap them freely.
package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// bare returns err as its operation and underlying cause only. The *os.PathError and
// *os.LinkError texts carry absolute paths; the cause (for example fs.ErrPermission) stays
// reachable with errors.Is. Other errors pass through.
func bare(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %w", pe.Op, pe.Err)
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return fmt.Errorf("%s: %w", le.Op, le.Err)
	}
	return err
}

// MkdirAll creates dir and any missing parents, like os.MkdirAll, with an error free of paths.
func MkdirAll(dir string, perm os.FileMode) error {
	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("create folder: %w", bare(err))
	}
	return nil
}

// WriteSynced writes data to path through a temp file in the same directory: write, fsync, close,
// rename. A failure leaves no temp file behind.
func WriteSynced(path string, data []byte) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", bare(err))
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, bare(f.Close()), bare(os.Remove(f.Name())))
		}
	}()
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("write file: %w", bare(err))
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", bare(err))
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close file: %w", bare(err))
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("rename file: %w", bare(err))
	}
	return nil
}
