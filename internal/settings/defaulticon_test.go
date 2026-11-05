package settings

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
)

func TestDrawDefaultIconUsesAccent(t *testing.T) {
	for _, size := range []int{512, 180} {
		b, err := drawDefaultIcon(size, "#AA3366")
		if err != nil {
			t.Fatalf("drawDefaultIcon(%d): %v", size, err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("decode %d: %v", size, err)
		}
		if got := img.Bounds().Dx(); got != size {
			t.Errorf("width = %d, want %d", got, size)
		}
		if got := color.RGBAModel.Convert(img.At(0, 0)); got != (color.RGBA{0xaa, 0x33, 0x66, 0xff}) {
			t.Errorf("%d corner = %v, want the accent", size, got)
		}
		if got := color.RGBAModel.Convert(img.At(size/2, size/2)); got != defaultIconCenter {
			t.Errorf("%d center = %v, want the light disc", size, got)
		}
	}
}

func TestDrawDefaultIconRejectsBadAccent(t *testing.T) {
	for _, accentColor := range []string{"", "#abc", "aa3366", "#gg3366"} {
		if _, err := drawDefaultIcon(64, accentColor); err == nil {
			t.Errorf("drawDefaultIcon(%q) = nil error", accentColor)
		}
	}
}
