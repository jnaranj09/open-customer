//go:build db

package settings

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// pngOf builds a solid-colour PNG of the given size.
func pngOf(t *testing.T, size int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for i := range img.Pix {
		img.Pix[i] = 0
	}
	img.Set(0, 0, c)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func brandingEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, "branding"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read branding: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSetIconsRejectsNonPNG(t *testing.T) {
	dir := t.TempDir()
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: dir})
	if _, err := m.Create(t.Context(), pool, validInput()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	good := pngOf(t, 4, color.RGBA{1, 2, 3, 255})
	for name, args := range map[string][2][]byte{
		"512 is not a PNG": {[]byte("GIF89a not a png"), good},
		"180 is not a PNG": {good, []byte("\xff\xd8\xff jpeg")},
		"512 is empty":     {nil, good},
	} {
		t.Run(name, func(t *testing.T) {
			err := m.SetIcons(t.Context(), pool, args[0], args[1])
			if !errors.As(err, &problem.UnsupportedMediaType{}) {
				t.Fatalf("err = %v, want UnsupportedMediaType", err)
			}
			if got := brandingEntries(t, dir); len(got) != 0 {
				t.Errorf("files written: %v", got)
			}
			if s, _ := m.Get(t.Context(), pool); s.HasCustomIcon {
				t.Error("has_custom_icon set after a rejected upload")
			}
		})
	}
}

func TestSetIconsRejectsOversize(t *testing.T) {
	dir := t.TempDir()
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: dir})
	if _, err := m.Create(t.Context(), pool, validInput()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	good := pngOf(t, 4, color.RGBA{1, 2, 3, 255})
	big := append([]byte(pngSignature), bytes.Repeat([]byte{0}, maxIconBytes)...)
	atLimit := append([]byte(pngSignature), bytes.Repeat([]byte{0}, maxIconBytes-len(pngSignature))...)
	for name, args := range map[string][2][]byte{
		"512 is over 1 MB": {big, good},
		"180 is over 1 MB": {good, big},
	} {
		t.Run(name, func(t *testing.T) {
			err := m.SetIcons(t.Context(), pool, args[0], args[1])
			if !errors.As(err, &problem.PayloadTooLarge{}) {
				t.Fatalf("err = %v, want PayloadTooLarge", err)
			}
			if got := brandingEntries(t, dir); len(got) != 0 {
				t.Errorf("files written: %v", got)
			}
		})
	}
	t.Run("exactly 1 MB is accepted", func(t *testing.T) {
		if err := m.SetIcons(t.Context(), pool, atLimit, good); err != nil {
			t.Fatalf("SetIcons: %v", err)
		}
	})
}

func TestSetIconsBeforeSetupIsNotFound(t *testing.T) {
	dir := t.TempDir()
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: dir})
	good := pngOf(t, 4, color.RGBA{1, 2, 3, 255})
	if err := m.SetIcons(t.Context(), pool, good, good); !errors.As(err, &problem.NotFound{}) {
		t.Fatalf("err = %v, want NotFound", err)
	}
	if got := brandingEntries(t, dir); len(got) != 0 {
		t.Errorf("files written: %v", got)
	}
}

func TestSetIconsThenIconServesUpload(t *testing.T) {
	dir := t.TempDir()
	pool := dbtest.Pool(t)
	fake := clock.NewFake(start)
	m := New(Options{Clock: fake, MediaDir: dir})
	if _, err := m.Create(t.Context(), pool, validInput()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	i512 := pngOf(t, 512, color.RGBA{9, 8, 7, 255})
	i180 := pngOf(t, 180, color.RGBA{6, 5, 4, 255})
	later := start.Add(time.Hour)
	fake.Set(later)
	if err := m.SetIcons(t.Context(), pool, i512, i180); err != nil {
		t.Fatalf("SetIcons: %v", err)
	}
	for size, want := range map[int][]byte{512: i512, 180: i180} {
		got, updated, err := m.Icon(t.Context(), pool, size)
		if err != nil {
			t.Fatalf("Icon(%d): %v", size, err)
		}
		if !bytes.Equal(got, want) || !updated.Equal(later) {
			t.Errorf("Icon(%d): bytes equal %v, updated %v; want the upload at %v", size, bytes.Equal(got, want), updated, later)
		}
	}
	if s, err := m.Get(t.Context(), pool); err != nil || !s.HasCustomIcon {
		t.Errorf("Get = %+v, %v; want has_custom_icon", s, err)
	}
	if got := brandingEntries(t, dir); len(got) != 2 {
		t.Errorf("branding files = %v, want exactly icon-512.png and icon-180.png (no temp files)", got)
	}
}

func TestSetIconsReplacesEarlierUpload(t *testing.T) {
	dir := t.TempDir()
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: dir})
	if _, err := m.Create(t.Context(), pool, validInput()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	first := pngOf(t, 4, color.RGBA{1, 1, 1, 255})
	second := pngOf(t, 4, color.RGBA{2, 2, 2, 255})
	for _, b := range [][]byte{first, second} {
		if err := m.SetIcons(t.Context(), pool, b, b); err != nil {
			t.Fatalf("SetIcons: %v", err)
		}
	}
	got, _, err := m.Icon(t.Context(), pool, 512)
	if err != nil || !bytes.Equal(got, second) {
		t.Errorf("Icon = equal %v, %v; want the second upload", bytes.Equal(got, second), err)
	}
}

func TestIconDefaultWhenNone(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	if _, err := m.Create(t.Context(), pool, validInput()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	var logs bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	for _, size := range []int{512, 180} {
		b, updated, err := m.Icon(t.Context(), pool, size)
		if err != nil {
			t.Fatalf("Icon(%d): %v", size, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(b))
		if err != nil || cfg.Width != size || cfg.Height != size {
			t.Errorf("Icon(%d) decodes as %dx%d, %v", size, cfg.Width, cfg.Height, err)
		}
		if !updated.IsZero() {
			t.Errorf("Icon(%d) time = %v, want zero for the default", size, updated)
		}
	}
	if logs.Len() != 0 {
		t.Errorf("log = %q, want nothing when no custom icon was set", logs.String())
	}
}

func TestIconDefaultBeforeSetup(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	b, updated, err := m.Icon(t.Context(), pool, 512)
	if err != nil || !updated.IsZero() {
		t.Fatalf("Icon = %v, %v", updated, err)
	}
	if cfg, err := png.DecodeConfig(bytes.NewReader(b)); err != nil || cfg.Width != 512 {
		t.Errorf("not the 512 default: %+v, %v", cfg, err)
	}
}

func TestIconUnknownSize(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	for _, size := range []int{0, 1, 192, 513, -512} {
		if _, _, err := m.Icon(t.Context(), pool, size); !errors.As(err, &problem.NotFound{}) {
			t.Errorf("Icon(%d) err = %v, want NotFound", size, err)
		}
	}
}

func TestIconMissingFileFallsBack(t *testing.T) {
	dir := t.TempDir()
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: dir})
	if _, err := m.Create(t.Context(), pool, validInput()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	good := pngOf(t, 4, color.RGBA{1, 2, 3, 255})
	if err := m.SetIcons(t.Context(), pool, good, good); err != nil {
		t.Fatalf("SetIcons: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "branding", "icon-512.png")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	var logs bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))

	b, updated, err := m.Icon(t.Context(), pool, 512)
	if err != nil {
		t.Fatalf("Icon: %v", err)
	}
	if cfg, err := png.DecodeConfig(bytes.NewReader(b)); err != nil || cfg.Width != 512 || !updated.IsZero() {
		t.Errorf("want the 512 default with zero time; got %+v, %v, %v", cfg, err, updated)
	}
	if !strings.Contains(logs.String(), "code=icon_file_missing") {
		t.Errorf("log = %q, want code=icon_file_missing", logs.String())
	}
	if strings.Contains(logs.String(), dir) {
		t.Errorf("log leaks the media path: %q", logs.String())
	}
}
