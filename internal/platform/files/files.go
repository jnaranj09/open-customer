// Package files holds the durable file write shared by settings icons and media photos.
package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteSynced writes data to path through a temp file in the same directory: write, fsync, close,
// rename. A failure leaves no temp file behind.
func WriteSynced(path string, data []byte) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, f.Close(), os.Remove(f.Name()))
		}
	}()
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("rename file: %w", err)
	}
	return nil
}
