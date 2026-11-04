package attributes

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const maxKeyLen = 40

// reserved are the template variables that attribute keys must not shadow.
var reserved = map[string]bool{
	"name": true, "first_name": true, "phone": true, "notes": true,
	"date": true, "time": true, "services": true, "id": true,
}

// GenerateKey turns a label into a key matching ^[a-z][a-z0-9_]*$ that is not in taken and not
// reserved. Accents are dropped, other characters become "_", runs collapse, an empty result is
// "a", a leading digit gets "a_", and the base is cut to 40 characters before any "_2" suffix.
func GenerateKey(label string, taken map[string]bool) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(label) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r >= 'A' && r <= 'Z':
			b.WriteRune(unicode.ToLower(r))
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	base := collapseUnderscores(b.String())
	base = strings.Trim(base, "_")
	if base == "" {
		base = "a"
	}
	if base[0] >= '0' && base[0] <= '9' {
		base = "a_" + base
	}
	if len(base) > maxKeyLen {
		base = strings.TrimRight(base[:maxKeyLen], "_")
	}
	if !reserved[base] && !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		key := base + "_" + strconv.Itoa(n)
		if !taken[key] {
			return key
		}
	}
}

func collapseUnderscores(s string) string {
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return s
}
