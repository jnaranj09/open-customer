package settings

import (
	"bytes"
	"context"
	_ "embed" // the default icons are embedded PNG files.
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/files"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	pngSignature = "\x89PNG\r\n\x1a\n"
	maxIconBytes = 1 << 20
	iconLarge    = 512
	iconSmall    = 180
	brandingDir  = "branding"
)

//go:embed default-icon-512.png
var defaultIcon512 []byte

//go:embed default-icon-180.png
var defaultIcon180 []byte

// SetIcons checks both PNGs, writes them under <MediaDir>/branding/ and then records the upload
// time. The files are synced and renamed into place before the row changes, so a row never points
// at a missing file. It is problem.UnsupportedMediaType when a part lacks the PNG signature,
// problem.PayloadTooLarge when a part is over 1 MB, and problem.NotFound before setup.
func (m *Module) SetIcons(ctx context.Context, q db.DBTX, icon512, icon180 []byte) error {
	for _, b := range [][]byte{icon512, icon180} {
		if !bytes.HasPrefix(b, []byte(pngSignature)) {
			return problem.UnsupportedMediaType{}
		}
		if len(b) > maxIconBytes {
			return problem.PayloadTooLarge{}
		}
	}
	if _, err := m.Get(ctx, q); err != nil {
		return err
	}
	dir := filepath.Join(m.mediaDir, brandingDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create branding dir: %w", err)
	}
	if err := files.WriteSynced(filepath.Join(dir, iconFile(iconLarge)), icon512); err != nil {
		return fmt.Errorf("write large icon: %w", err)
	}
	if err := files.WriteSynced(filepath.Join(dir, iconFile(iconSmall)), icon180); err != nil {
		return fmt.Errorf("write small icon: %w", err)
	}
	// The Get above found the row, and nothing deletes it, so the update always hits it.
	if err := sqlc.New(q).SetIconUpdatedAt(ctx, m.clock.Now()); err != nil {
		return fmt.Errorf("record icons: %w", err)
	}
	return nil
}

// Icon returns the PNG for size 512 or 180 and the time it was uploaded. Without an upload (or
// before setup, or when the uploaded file is gone) it returns the built-in default with a zero
// time. Any other size is problem.NotFound.
func (m *Module) Icon(ctx context.Context, q db.DBTX, size int) ([]byte, time.Time, error) {
	var fallback []byte
	switch size {
	case iconLarge:
		fallback = defaultIcon512
	case iconSmall:
		fallback = defaultIcon180
	default:
		return nil, time.Time{}, problem.NotFound{}
	}
	row, err := sqlc.New(q).GetSettings(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return fallback, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("get icon settings: %w", err)
	}
	if row.IconUpdatedAt == nil {
		return fallback, time.Time{}, nil
	}
	b, err := os.ReadFile(filepath.Join(m.mediaDir, brandingDir, iconFile(size)))
	if errors.Is(err, os.ErrNotExist) {
		slog.ErrorContext(ctx, "custom icon file is missing; serving the default", "code", "icon_file_missing", "size", size)
		return fallback, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("read icon: %w", err)
	}
	return b, *row.IconUpdatedAt, nil
}

func iconFile(size int) string { return fmt.Sprintf("icon-%d.png", size) }
