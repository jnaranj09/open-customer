// Package color holds the one #rrggbb check shared by the settings accent color and the service
// colors.
package color

import (
	"regexp"
	"strings"
)

var pattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Normalize lowercases s and reports whether it is a #rrggbb color.
func Normalize(s string) (string, bool) {
	s = strings.ToLower(s)
	return s, pattern.MatchString(s)
}
