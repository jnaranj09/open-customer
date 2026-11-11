// Package text holds the one trim-and-length check shared by the modules that validate names,
// labels and notes, the one notes rule and the one optional one-line text rule.
package text

import (
	"strings"
	"unicode/utf8"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
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

// Line checks an optional one-line text (a service description, a quick reply title). It trims
// raw; blank gives nil (stored as NULL). msg is "" when the text is fine, else the field message:
// problem.MsgTextOneLine when it holds a line break, problem.MsgTextTooLong when it has more than
// maxRunes runes. The caller adds its own field name.
func Line(raw string, maxRunes int) (value *string, msg string) {
	s := strings.TrimSpace(raw)
	if strings.ContainsAny(s, "\r\n") {
		return nil, problem.MsgTextOneLine
	}
	if utf8.RuneCountInString(s) > maxRunes {
		return nil, problem.MsgTextTooLong
	}
	if s == "" {
		return nil, ""
	}
	return &s, ""
}
