// Package web holds the built SPA. It sits in web/ because go:embed cannot reach parent folders.
// web/build is gitignored: run `make web-stub` (or `make web`) before any Go command.
package web

import "embed"

// Build is the SvelteKit output under "build/".
//
//go:embed all:build
var Build embed.FS
