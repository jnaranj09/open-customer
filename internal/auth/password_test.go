package auth

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

func TestHashRoundTrip(t *testing.T) {
	t.Parallel()
	stored, err := hashPassword(t.Context(), "made-up-secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	ok, err := verifyPassword(t.Context(), stored, "made-up-secret")
	if err != nil || !ok {
		t.Fatalf("verifyPassword(own hash, same password) = %v, %v; want true, nil", ok, err)
	}
}

func TestHashFormatIsPHC(t *testing.T) {
	t.Parallel()
	stored, err := hashPassword(t.Context(), "made-up-secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	// 16-byte salt and 32-byte key in unpadded standard base64: 22 and 43 characters.
	phc := regexp.MustCompile(`^\$argon2id\$v=19\$m=65536,t=3,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)
	if !phc.MatchString(stored) {
		t.Fatalf("hashPassword = %q; want the PHC format %s", stored, phc)
	}
}

func TestHashUsesAFreshSalt(t *testing.T) {
	t.Parallel()
	first, err := hashPassword(t.Context(), "made-up-secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	second, err := hashPassword(t.Context(), "made-up-secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if first == second {
		t.Fatalf("two hashes of the same password are equal (%q); want a random salt each time", first)
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	t.Parallel()
	stored, err := hashPassword(t.Context(), "made-up-secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	ok, err := verifyPassword(t.Context(), stored, "made-up-secreT")
	if err != nil || ok {
		t.Fatalf("verifyPassword(wrong password) = %v, %v; want false, nil", ok, err)
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	t.Parallel()
	// A well-formed hash of nothing in particular: "saltsaltsaltsalt" and 32 zero bytes.
	const salt, key = "c2FsdHNhbHRzYWx0c2FsdA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	tests := []struct {
		name   string
		stored string
	}{
		{"a wrong algorithm prefix", "$argon2i$v=19$m=65536,t=3,p=1$" + salt + "$" + key},
		{"a wrong version", "$argon2id$v=18$m=65536,t=3,p=1$" + salt + "$" + key},
		{"a salt that is not base64", "$argon2id$v=19$m=65536,t=3,p=1$!!!!$" + key},
		{"a key that is not base64", "$argon2id$v=19$m=65536,t=3,p=1$" + salt + "$!!!!"},
		{"padded base64", "$argon2id$v=19$m=65536,t=3,p=1$" + salt + "==$" + key},
		{"a missing key part", "$argon2id$v=19$m=65536,t=3,p=1$" + salt},
		{"an extra part", "$argon2id$v=19$m=65536,t=3,p=1$" + salt + "$" + key + "$x"},
		{"an empty salt", "$argon2id$v=19$m=65536,t=3,p=1$$" + key},
		{"an empty key", "$argon2id$v=19$m=65536,t=3,p=1$" + salt + "$"},
		{"a key that is not 32 bytes", "$argon2id$v=19$m=65536,t=3,p=1$" + salt + "$AAAA"},
		{"m that is not a number", "$argon2id$v=19$m=lots,t=3,p=1$" + salt + "$" + key},
		{"m that does not fit in 32 bits", "$argon2id$v=19$m=4294967296,t=3,p=1$" + salt + "$" + key},
		{"p that does not fit in 8 bits", "$argon2id$v=19$m=65536,t=3,p=256$" + salt + "$" + key},
		{"a signed m", "$argon2id$v=19$m=+65536,t=3,p=1$" + salt + "$" + key},
		{"a missing parameter", "$argon2id$v=19$m=65536,t=3$" + salt + "$" + key},
		{"parameters out of order", "$argon2id$v=19$t=3,m=65536,p=1$" + salt + "$" + key},
		{"zero iterations", "$argon2id$v=19$m=65536,t=0,p=1$" + salt + "$" + key},
		{"zero threads", "$argon2id$v=19$m=65536,t=3,p=0$" + salt + "$" + key},
		{"an empty string", ""},
	}
	for _, tt := range tests {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			t.Parallel()
			ok, err := verifyPassword(t.Context(), tt.stored, "made-up-secret")
			if err == nil || ok {
				t.Fatalf("verifyPassword(%q) = %v, %v; want false and an error", tt.stored, ok, err)
			}
		})
	}
}

func TestVerifyHonoursStoredParams(t *testing.T) {
	t.Parallel()
	stored, err := hashWith(t.Context(), "made-up-secret", argonParams{memory: 65536, iterations: 1, threads: 1})
	if err != nil {
		t.Fatalf("hashWith: %v", err)
	}
	if !strings.HasPrefix(stored, "$argon2id$v=19$m=65536,t=1,p=1$") {
		t.Fatalf("hashWith(t=1) = %q; want t=1 in the parameters", stored)
	}
	ok, err := verifyPassword(t.Context(), stored, "made-up-secret")
	if err != nil || !ok {
		t.Fatalf("verifyPassword(t=1 hash, same password) = %v, %v; want true, nil", ok, err)
	}
}

func TestPasswordTooShort(t *testing.T) {
	t.Parallel()
	want := problem.Validation{Errors: []problem.FieldError{
		{Field: "new_password", Message: problem.MsgPasswordTooShort},
	}}

	t.Run("7 characters are rejected on the given field", func(t *testing.T) {
		t.Parallel()
		var got problem.Validation
		if err := validatePassword("new_password", "1234567"); !errors.As(err, &got) || !reflect.DeepEqual(got, want) {
			t.Fatalf("validatePassword(7 characters) = %v; want %+v", err, want)
		}
	})
	t.Run("7 multi-byte characters are rejected (the length counts characters, not bytes)", func(t *testing.T) {
		t.Parallel()
		var got problem.Validation
		if err := validatePassword("new_password", "ñññññññ"); !errors.As(err, &got) || !reflect.DeepEqual(got, want) {
			t.Fatalf("validatePassword(7 multi-byte characters) = %v; want %+v", err, want)
		}
	})
	t.Run("8 characters are accepted", func(t *testing.T) {
		t.Parallel()
		if err := validatePassword("new_password", "12345678"); err != nil {
			t.Fatalf("validatePassword(8 characters) = %v; want nil", err)
		}
	})
}

// An unknown username verifies against dummyHash (OP17); it must cost what a real hash costs, or
// the response time tells which usernames exist.
func TestDummyHashUsesDefaultParams(t *testing.T) {
	t.Parallel()
	p, _, _, err := parsePHC(dummyHash)
	if err != nil || p != defaultParams {
		t.Fatalf("parsePHC(dummyHash) = %+v, %v; want %+v, nil", p, err, defaultParams)
	}
}

// heldHashes replaces the argon2 call with one that counts the calls inside it and blocks until
// release is called, and gives the hash slots a fresh channel. Both are made inside the caller's
// synctest bubble, so synctest.Wait sees a goroutine blocked on either as durably blocked. The
// cleanup releases too, so a failed test does not leave goroutines blocked. The test must not be
// parallel: it swaps package state the parallel tests use.
func heldHashes(t *testing.T) (inside *atomic.Int32, release func()) {
	t.Helper()
	inside, held := new(atomic.Int32), make(chan struct{})
	release = sync.OnceFunc(func() { close(held) })
	realKey, realSlots := argonIDKey, hashSlots
	argonIDKey = func(_, _ []byte, _, _ uint32, _ uint8, keyLen uint32) []byte {
		inside.Add(1)
		<-held
		return make([]byte, keyLen)
	}
	hashSlots = make(chan struct{}, maxConcurrentHashes)
	t.Cleanup(func() {
		release()
		argonIDKey, hashSlots = realKey, realSlots
	})
	return inside, release
}

// Each hash or verify holds 64 MiB, so at most maxConcurrentHashes run at once; the rest wait.
func TestHashConcurrencyIsCapped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		inside, release := heldHashes(t)
		var done sync.WaitGroup
		for range 3 {
			done.Go(func() { _, _ = hashPassword(t.Context(), "made-up-secret") })
			done.Go(func() { _, _ = verifyPassword(t.Context(), dummyHash, "made-up-secret") })
		}
		synctest.Wait()
		if got := inside.Load(); got != maxConcurrentHashes {
			t.Fatalf("6 hashes and verifies at once: %d inside argon2; want %d", got, maxConcurrentHashes)
		}
		release()
		done.Wait()
		if got := inside.Load(); got != 6 {
			t.Fatalf("after release: %d argon2 calls; want all 6", got)
		}
	})
}

// A hash waiting for a slot gives up when its context ends, and runs no argon2 at all.
func TestHashWaitsReturnsOnCtxDone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		inside, _ := heldHashes(t)
		for range maxConcurrentHashes {
			hashSlots <- struct{}{}
		}
		ctx, cancel := context.WithCancel(t.Context())
		var err error
		returned := make(chan struct{})
		go func() {
			_, err = hashPassword(ctx, "made-up-secret")
			close(returned)
		}()
		synctest.Wait()
		if got := inside.Load(); got != 0 {
			t.Fatalf("every slot taken: %d argon2 calls; want 0 (the hash waits)", got)
		}
		select {
		case <-returned:
			t.Fatalf("hashPassword returned (%v) while every slot was taken; want it waiting", err)
		default:
		}
		cancel()
		<-returned
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("hashPassword after the context ended = %v; want context.Canceled", err)
		}
		if got := inside.Load(); got != 0 {
			t.Fatalf("argon2 calls after the context ended = %d; want 0 (no hash runs without a slot)", got)
		}
	})
}
