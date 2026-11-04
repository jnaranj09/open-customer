// Package text holds the one trim-and-length check shared by the modules that validate names,
// labels and notes, and the one notes rule.
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

// MaxNotes is the most runes a notes text (clients, appointments) may have.
const MaxNotes = 2000

// Notes trims optional notes. nil or blank gives nil (stored as NULL). ok is false when the
// trimmed text has more than MaxNotes runes; the caller adds its own field error.
func Notes(raw *string) (notes *string, ok bool) {
	if raw == nil {
		return nil, true
	}
	s, ok := Trim(*raw, 0, MaxNotes)
	if !ok {
		return nil, false
	}
	if s == "" {
		return nil, true
	}
	return &s, true
}
