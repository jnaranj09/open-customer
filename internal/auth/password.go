package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// minPasswordLength is counted in characters (runes), not bytes (item 38).
const minPasswordLength = 8

const (
	saltLength = 16
	keyLength  = 32
)

// argonParams are the Argon2id cost parameters written into every PHC string.
type argonParams struct {
	memory     uint32 // KiB
	iterations uint32
	threads    uint8
}

// defaultParams is item 38: m=64 MiB, t=3, p=1.
var defaultParams = argonParams{memory: 64 * 1024, iterations: 3, threads: 1}

// maxConcurrentHashes caps how many argon2id hashes and verifies run at once: each holds
// defaultParams.memory (64 MiB) while it runs, and the target box is small (item 41). A burst of
// logins for different usernames, each within its own delay, would otherwise hold that much
// memory per request.
const maxConcurrentHashes = 2

// hashSlots is the semaphore behind maxConcurrentHashes: a hash or verify sends into it before
// argon2 runs and receives after. It is package-level because the cap is per process.
var hashSlots = make(chan struct{}, maxConcurrentHashes)

// argonIDKey is argon2.IDKey. It is a variable only so a test can count the calls inside it while
// they run (the concurrency cap has no other trace).
var argonIDKey = argon2.IDKey

// withHashSlot runs fn once a hash slot is free. If ctx ends first it returns ctx's error and fn
// never runs. An already ended ctx never takes a slot, even when one is free.
func withHashSlot(ctx context.Context, fn func()) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("wait for a password hash slot: %w", err)
	}
	select {
	case hashSlots <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("wait for a password hash slot: %w", ctx.Err())
	}
	defer func() { <-hashSlots }()
	fn()
	return nil
}

// errMalformedHash means a stored password hash is not a PHC string this package can verify. It
// never carries the stored value.
var errMalformedHash = errors.New("malformed password hash")

// hashPassword returns pw hashed with Argon2id and the default parameters, as a PHC string:
// $argon2id$v=19$m=65536,t=3,p=1$<salt>$<hash>. It waits for a hash slot (maxConcurrentHashes).
func hashPassword(ctx context.Context, pw string) (string, error) {
	return hashWith(ctx, pw, defaultParams)
}

// hashWith hashes pw with the given parameters and a fresh random salt, in a hash slot.
func hashWith(ctx context.Context, pw string, p argonParams) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read a password salt: %w", err)
	}
	var key []byte
	if err := withHashSlot(ctx, func() {
		key = argonIDKey([]byte(pw), salt, p.iterations, p.memory, p.threads, keyLength)
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, p.memory, p.iterations,
		p.threads, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// verifyPassword reports whether pw matches the stored PHC string. It recomputes the key with the
// stored parameters, so a hash made with older parameters still verifies. A stored value it cannot
// parse is an error, never a match. Like hashPassword it waits for a hash slot; a malformed
// stored value is refused before it takes one.
func verifyPassword(ctx context.Context, stored, pw string) (bool, error) {
	p, salt, key, err := parsePHC(stored)
	if err != nil {
		return false, err
	}
	var got []byte
	if err := withHashSlot(ctx, func() {
		got = argonIDKey([]byte(pw), salt, p.iterations, p.memory, p.threads, keyLength)
	}); err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, key) == 1, nil
}

// parsePHC splits $argon2id$v=19$m=<m>,t=<t>,p=<p>$<salt>$<key> into its parts.
func parsePHC(stored string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(stored, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return argonParams{}, nil, nil, fmt.Errorf("%w: header", errMalformedHash)
	}
	p, err := parseParams(parts[3])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return argonParams{}, nil, nil, fmt.Errorf("%w: salt", errMalformedHash)
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) != keyLength {
		return argonParams{}, nil, nil, fmt.Errorf("%w: key", errMalformedHash)
	}
	return p, salt, key, nil
}

// parseParams reads "m=<m>,t=<t>,p=<p>" in that order. Zero iterations or threads would make
// argon2 panic, so they are rejected here, as is a value too large for its argon2 type.
func parseParams(s string) (argonParams, error) {
	fields := strings.Split(s, ",")
	if len(fields) != 3 {
		return argonParams{}, fmt.Errorf("%w: parameters", errMalformedHash)
	}
	m, errM := parseParam(fields[0], "m=")
	t, errT := parseParam(fields[1], "t=")
	p, errP := parseParam(fields[2], "p=")
	if errM != nil || errT != nil || errP != nil ||
		m > math.MaxUint32 || t < 1 || t > math.MaxUint32 || p < 1 || p > math.MaxUint8 {
		return argonParams{}, fmt.Errorf("%w: parameters", errMalformedHash)
	}
	return argonParams{memory: uint32(m), iterations: uint32(t), threads: uint8(p)}, nil
}

// parseParam reads one "<name>=<unsigned decimal>" field.
func parseParam(field, prefix string) (uint64, error) {
	digits, ok := strings.CutPrefix(field, prefix)
	if !ok {
		return 0, errMalformedHash
	}
	n, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, errMalformedHash
	}
	return n, nil
}

// validatePassword checks the minimum length; field names the input in the Validation error.
func validatePassword(field, pw string) error {
	if utf8.RuneCountInString(pw) < minPasswordLength {
		return problem.Field(field, problem.MsgPasswordTooShort)
	}
	return nil
}
