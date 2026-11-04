// Package text holds the one trim-and-length check shared by the modules that validate names,
// labels and notes.
package text

import (
	"strings"
	"unicode/utf8"
)

// Trim trims surrounding spaces and reports whether the rune count of the result is in
// [lo, hi]. The caller adds its own field error and message.
func Trim(s string, lo, hi int) (string, bool) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	return s, n >= lo && n <= hi
}
