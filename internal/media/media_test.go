package media

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// testJPEG returns a real JPEG of the given size.
func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func unitModule(t *testing.T) *Module {
	t.Helper()
	return New(Options{Clock: clock.NewFake(time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)), MediaDir: t.TempDir()})
}

func TestSaveRejectsNonJPEG(t *testing.T) {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	good := testJPEG(t, 8, 8)
	tests := []struct {
		name        string
		full, thumb []byte
	}{
		{"png full", pngBuf.Bytes(), good},
		{"png thumb", good, pngBuf.Bytes()},
		{"empty full", nil, good},
		{"oversize png full", append(append([]byte{}, pngBuf.Bytes()[:8]...), make([]byte, MaxFullBytes)...), good},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := unitModule(t).Save(t.Context(), nil, MediaInput{Full: tc.full, Thumb: tc.thumb})
			if !errors.As(err, new(problem.UnsupportedMediaType)) {
				t.Fatalf("err = %v; want UnsupportedMediaType", err)
			}
		})
	}
}

func TestSaveRejectsOversize(t *testing.T) {
	good := testJPEG(t, 8, 8)
	big := func(n int) []byte {
		b := make([]byte, n)
		copy(b, []byte{0xFF, 0xD8, 0xFF})
		return b
	}
	tests := []struct {
		name        string
		full, thumb []byte
	}{
		{"full", big(MaxFullBytes + 1), good},
		{"thumb", good, big(MaxThumbBytes + 1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := unitModule(t).Save(t.Context(), nil, MediaInput{Full: tc.full, Thumb: tc.thumb})
			if !errors.As(err, new(problem.PayloadTooLarge)) {
				t.Fatalf("err = %v; want PayloadTooLarge", err)
			}
		})
	}
}

func TestSaveRejectsUnreadableJPEG(t *testing.T) {
	good := testJPEG(t, 8, 8)
	bad := []byte{0xFF, 0xD8, 0xFF, 0x00, 0x00, 0x00}
	tests := []struct {
		name        string
		full, thumb []byte
	}{
		{"full", bad, good},
		{"thumb", good, bad},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := unitModule(t).Save(t.Context(), nil, MediaInput{Full: tc.full, Thumb: tc.thumb})
			if !errors.As(err, new(problem.UnsupportedMediaType)) {
				t.Fatalf("err = %v; want UnsupportedMediaType", err)
			}
		})
	}
}
