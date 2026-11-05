package settings

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"math"

	accent "github.com/jnaranj09/open-customer/internal/platform/color"
)

// defaultIconCenter is the light disc of the built-in icon: the web theme's `--bg-100`, so the
// icon matches the page behind it. The embedded PNGs (used before setup) have the same shape in
// the default accent.
var defaultIconCenter = color.RGBA{R: 0xfb, G: 0xf5, B: 0xf2, A: 0xff}

// defaultIconRadius is the light disc's radius as a share of the icon's side.
const defaultIconRadius = 0.3

// drawDefaultIcon draws the built-in icon at size px: a square in the accent color with a light
// disc in the middle. The disc edge is anti-aliased over one pixel. A bad accent is an error.
func drawDefaultIcon(size int, accentColor string) ([]byte, error) {
	fill, err := parseAccent(accentColor)
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	center := float64(size) / 2
	radius := defaultIconRadius * float64(size)
	for y := range size {
		for x := range size {
			d := math.Hypot(float64(x)+0.5-center, float64(y)+0.5-center)
			inside := math.Max(0, math.Min(1, radius-d+0.5))
			img.SetRGBA(x, y, mix(fill, defaultIconCenter, inside))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode default icon: %w", err)
	}
	return buf.Bytes(), nil
}

// accentDefaultIcon is the built-in icon in the business's accent color. A stored accent that does
// not parse logs code=accent_color_invalid and serves fallback, the embedded default.
func accentDefaultIcon(ctx context.Context, size int, accentColor string, fallback []byte) []byte {
	b, err := drawDefaultIcon(size, accentColor)
	if err != nil {
		slog.ErrorContext(ctx, "default icon not drawn; serving the embedded one", "code", "accent_color_invalid", "size", size)
		return fallback
	}
	return b
}

func parseAccent(s string) (color.RGBA, error) {
	s, ok := accent.Normalize(s)
	if !ok {
		return color.RGBA{}, fmt.Errorf("accent color is not #rrggbb")
	}
	rgb, err := hex.DecodeString(s[1:])
	if err != nil {
		return color.RGBA{}, fmt.Errorf("parse accent color: %w", err)
	}
	return color.RGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 0xff}, nil
}

// mix blends a toward b by t (0 = a, 1 = b).
func mix(a, b color.RGBA, t float64) color.RGBA {
	ch := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.RGBA{R: ch(a.R, b.R), G: ch(a.G, b.G), B: ch(a.B, b.B), A: 0xff}
}
