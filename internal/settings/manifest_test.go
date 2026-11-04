//go:build db

package settings

import (
	"encoding/json"
	"testing"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

type manifestDoc struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	ThemeColor      *string        `json:"theme_color"`
	BackgroundColor *string        `json:"background_color"`
	Lang            string         `json:"lang"`
	Icons           []manifestIcon `json:"icons"`
}

func parseManifest(t *testing.T, b []byte) manifestDoc {
	t.Helper()
	var d manifestDoc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("manifest is not JSON: %v", err)
	}
	return d
}

var wantIcons = []manifestIcon{{Src: "/icons/512.png", Sizes: "512x512", Type: "image/png", Purpose: "any"}}

func TestManifestFromSettings(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	in := validInput()
	in.Name = "Ñandú Studio Paris" // 18 runes, non-ASCII
	if _, err := m.Create(t.Context(), pool, in); err != nil {
		t.Fatalf("Create: %v", err)
	}
	b, err := m.Manifest(t.Context(), pool)
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	d := parseManifest(t, b)
	if d.Name != in.Name || d.ShortName != "Ñandú Studio" {
		t.Errorf("name %q short_name %q, want the full name and its first 12 runes", d.Name, d.ShortName)
	}
	if d.StartURL != "/" || d.Scope != "/" || d.Display != "standalone" || d.Lang != "es" {
		t.Errorf("fixed fields wrong: %+v", d)
	}
	if d.ThemeColor == nil || *d.ThemeColor != "#aa3366" || d.BackgroundColor == nil || *d.BackgroundColor != "#aa3366" {
		t.Errorf("colors = %v, %v; want the accent", d.ThemeColor, d.BackgroundColor)
	}
	if len(d.Icons) != 1 || d.Icons[0] != wantIcons[0] {
		t.Errorf("icons = %+v, want %+v", d.Icons, wantIcons)
	}
}

func TestManifestShortNameKeepsShortName(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	in := validInput()
	in.Name = "Salón Ñu" // 8 runes, 10 bytes
	if _, err := m.Create(t.Context(), pool, in); err != nil {
		t.Fatalf("Create: %v", err)
	}
	b, err := m.Manifest(t.Context(), pool)
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	if d := parseManifest(t, b); d.ShortName != in.Name {
		t.Errorf("short_name = %q, want %q", d.ShortName, in.Name)
	}
}

func TestManifestBeforeSetup(t *testing.T) {
	pool := dbtest.Pool(t)
	m := New(Options{Clock: clock.NewFake(start), MediaDir: t.TempDir()})
	b, err := m.Manifest(t.Context(), pool)
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	d := parseManifest(t, b)
	if d.Name != "open-customer" || d.ShortName != "open-customer" || d.Lang != "es" {
		t.Errorf("names/lang = %q, %q, %q", d.Name, d.ShortName, d.Lang)
	}
	if d.ThemeColor != nil || d.BackgroundColor != nil {
		t.Errorf("colors = %v, %v; want none before setup", d.ThemeColor, d.BackgroundColor)
	}
	if len(d.Icons) != 1 || d.Icons[0] != wantIcons[0] {
		t.Errorf("icons = %+v, want the default icon entry", d.Icons)
	}
}
