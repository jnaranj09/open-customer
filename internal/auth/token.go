package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// tokenBytes is the size of a setup or session token ("Session and request rules").
const tokenBytes = 32

// tokenEncoding writes a token as unpadded base64url (43 characters). Strict rejects non-zero
// trailing bits, so one token has exactly one spelling.
var tokenEncoding = base64.RawURLEncoding.Strict()

// tokenLength is the encoded length; checking it first also rejects the CR and LF that the
// decoder would otherwise skip.
var tokenLength = tokenEncoding.EncodedLen(tokenBytes)

// newToken returns a fresh random token, as sent to the browser, and the hash that is stored.
func newToken() (string, []byte, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("read a token: %w", err)
	}
	return tokenEncoding.EncodeToString(raw), hashToken(raw), nil
}

// parseToken is the one parse site for a token from outside. Anything that does not decode to
// exactly 32 bytes is Unauthorized (setup answers SetupTokenInvalid instead). It returns the stored
// hash, never the raw bytes.
func parseToken(s string) ([]byte, error) {
	if len(s) != tokenLength {
		return nil, problem.Unauthorized{}
	}
	raw, err := tokenEncoding.DecodeString(s)
	if err != nil || len(raw) != tokenBytes {
		return nil, problem.Unauthorized{}
	}
	return hashToken(raw), nil
}

// hashToken is the stored form: sha256 of the raw 32 bytes.
func hashToken(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}
