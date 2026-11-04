package auth

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

func TestParseToken(t *testing.T) {
	t.Parallel()
	token, hash, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}

	t.Run("a new token is 43 characters and parses to its stored hash", func(t *testing.T) {
		t.Parallel()
		got, err := parseToken(token)
		if len(token) != 43 || err != nil || !bytes.Equal(got, hash) {
			t.Fatalf("parseToken(%d-char new token) = %x, %v; want %x, nil", len(token), got, err, hash)
		}
	})
	t.Run("the stored hash is sha256 of the 32 raw bytes", func(t *testing.T) {
		t.Parallel()
		raw, err := tokenEncoding.DecodeString(token)
		sum := sha256.Sum256(raw)
		if err != nil || len(raw) != 32 || !bytes.Equal(hash, sum[:]) {
			t.Fatalf("hash = %x for %d raw bytes (%v); want sha256 %x", hash, len(raw), err, sum)
		}
	})

	rejected := []struct {
		name  string
		token string
	}{
		{"an empty token", ""},
		{"a short token", "abc"},
		{"a 44-character token", token + "A"},
		{"43 characters with a line feed that decode to 31 bytes", strings.Repeat("A", 21) + "\n" + strings.Repeat("A", 21)},
		{"a token with a character outside base64url", "/" + token[1:]},
		{"the same token with non-zero trailing bits", flipTrailingBits(token)},
		{"43 spaces", strings.Repeat(" ", 43)},
	}
	for _, tt := range rejected {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			t.Parallel()
			var u problem.Unauthorized
			if got, err := parseToken(tt.token); !errors.As(err, &u) || got != nil {
				t.Fatalf("parseToken(%q) = %x, %v; want nil, problem.Unauthorized", tt.token, got, err)
			}
		})
	}
}

// flipTrailingBits sets the lowest bit of the last character. 32 bytes in 43 characters leave two
// unused bits there, so a lenient decoder reads the result as the same 32 bytes.
func flipTrailingBits(token string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, token[len(token)-1])
	return token[:len(token)-1] + string(alphabet[last|1])
}
