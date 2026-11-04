package auth

import (
	"regexp"
	"strings"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// usernamePattern is the rule from "Session and request rules": 3–32 of a–z, 0–9, '.', '_', '-'.
var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

// normalizeUsername trims and lowercases s, then checks the username rule. Setup and login both
// call it, so " Ana " logs in as "ana".
func normalizeUsername(s string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(s))
	if !usernamePattern.MatchString(name) {
		return "", problem.Field("username", problem.MsgUsernameInvalid)
	}
	return name, nil
}
