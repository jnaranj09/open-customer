// Package templates parses and renders message templates (design doc "Message templates"). It is
// pure: no database, no clock. Parse and Render share one scanner, strict in Parse and lenient in
// Render.
package templates

import (
	"fmt"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// VarRef is one well-formed {{ns.key}} or {{ns.key | "fallback"}} token.
type VarRef struct {
	NS, Key  string
	Fallback *string
	// Pos is the 1-based rune offset of the token's "{{".
	Pos int
}

// Parsed holds every well-formed variable of a body, in body order.
type Parsed struct{ Vars []VarRef }

// segment is one piece of a scanned body: literal text, a variable, or a "{{" that starts no
// valid variable (bad, with its position).
type segment struct {
	lit string
	ref *VarRef
	bad int
}

// Parse checks a template body. The error is a problem.Validation with one MsgTemplateUnclosed on
// field "body" per "{{" that starts no valid variable, in body order. Parsed is returned with the
// error and holds every well-formed variable, so the caller can still check them.
func Parse(body string) (Parsed, error) {
	var parsed Parsed
	var errs []problem.FieldError
	for _, s := range scan(body) {
		switch {
		case s.ref != nil:
			parsed.Vars = append(parsed.Vars, *s.ref)
		case s.bad > 0:
			errs = append(errs, problem.FieldError{Field: "body", Message: fmt.Sprintf(problem.MsgTemplateUnclosed, s.bad)})
		}
	}
	if len(errs) > 0 {
		return parsed, problem.Validation{Errors: errs}
	}
	return parsed, nil
}

// scan splits body into segments with the grammar:
//
//	token    := "{{" sp* ns "." key sp* ( "|" sp* '"' fallback '"' sp* )? "}}"
//	ns       := [a-z]+          key := [a-z][a-z0-9_]*
//	fallback := any rune except '"'     sp := ' ' (ASCII space only)
//
// A "{{" that does not match is a bad segment, and scanning resumes after those two characters.
// Anything outside a "{{" is literal text.
func scan(body string) []segment {
	rs := []rune(body)
	var segs []segment
	litStart := 0
	flush := func(end int) {
		if end > litStart {
			segs = append(segs, segment{lit: string(rs[litStart:end])})
		}
	}
	for i := 0; i < len(rs); {
		if rs[i] != '{' || i+1 >= len(rs) || rs[i+1] != '{' {
			i++
			continue
		}
		flush(i)
		if ref, end, ok := matchToken(rs, i); ok {
			segs = append(segs, segment{ref: &ref})
			i = end
		} else {
			segs = append(segs, segment{bad: i + 1})
			i += 2
		}
		litStart = i
	}
	flush(len(rs))
	return segs
}

// matchToken matches one token whose "{{" is at rs[start]. end is the index after its "}}".
func matchToken(rs []rune, start int) (ref VarRef, end int, ok bool) {
	i := skipSpaces(rs, start+2)
	nsStart := i
	for i < len(rs) && isLower(rs[i]) {
		i++
	}
	if i == nsStart || i >= len(rs) || rs[i] != '.' {
		return VarRef{}, 0, false
	}
	ref = VarRef{NS: string(rs[nsStart:i]), Pos: start + 1}
	i++
	keyStart := i
	if i >= len(rs) || !isLower(rs[i]) {
		return VarRef{}, 0, false
	}
	for i < len(rs) && (isLower(rs[i]) || isDigit(rs[i]) || rs[i] == '_') {
		i++
	}
	ref.Key = string(rs[keyStart:i])
	i = skipSpaces(rs, i)
	if i < len(rs) && rs[i] == '|' {
		i = skipSpaces(rs, i+1)
		if i >= len(rs) || rs[i] != '"' {
			return VarRef{}, 0, false
		}
		i++
		fbStart := i
		for i < len(rs) && rs[i] != '"' {
			i++
		}
		if i >= len(rs) {
			return VarRef{}, 0, false
		}
		fb := string(rs[fbStart:i])
		ref.Fallback = &fb
		i = skipSpaces(rs, i+1)
	}
	if i+1 >= len(rs) || rs[i] != '}' || rs[i+1] != '}' {
		return VarRef{}, 0, false
	}
	return ref, i + 2, true
}

func skipSpaces(rs []rune, i int) int {
	for i < len(rs) && rs[i] == ' ' {
		i++
	}
	return i
}

func isLower(r rune) bool { return r >= 'a' && r <= 'z' }
func isDigit(r rune) bool { return r >= '0' && r <= '9' }
