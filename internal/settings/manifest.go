package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	shortNameRunes = 12
	// beforeSetupName is the manifest name while no settings row exists.
	beforeSetupName = "open-customer"
)

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

type manifest struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	ThemeColor      string         `json:"theme_color,omitempty"`
	BackgroundColor string         `json:"background_color,omitempty"`
	Lang            string         `json:"lang"`
	Icons           []manifestIcon `json:"icons"`
}

// Manifest returns the web app manifest JSON built from the settings. Before setup it is a minimal
// manifest: the name "open-customer", Spanish, no colors and the default icons.
func (m *Module) Manifest(ctx context.Context, q db.DBTX) ([]byte, error) {
	doc := manifest{
		Name: beforeSetupName, ShortName: beforeSetupName, Lang: supportedLocale,
		StartURL: "/", Scope: "/", Display: "standalone",
		Icons: []manifestIcon{{Src: "/icons/512.png", Sizes: "512x512", Type: "image/png", Purpose: "any"}},
	}
	s, err := m.Get(ctx, q)
	switch {
	case errors.As(err, &problem.NotFound{}):
	case err != nil:
		return nil, fmt.Errorf("manifest: %w", err)
	default:
		doc.Name, doc.ShortName, doc.Lang = s.Name, shortName(s.Name), s.Locale
		doc.ThemeColor, doc.BackgroundColor = s.AccentColor, s.AccentColor
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return out, nil
}

// shortName cuts name to its first shortNameRunes runes.
func shortName(name string) string {
	runes := []rune(name)
	if len(runes) <= shortNameRunes {
		return name
	}
	return string(runes[:shortNameRunes])
}
