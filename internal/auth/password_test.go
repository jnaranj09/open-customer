package auth

import (
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

func TestHashRoundTrip(t *testing.T) {
	t.Parallel()
	stored, err := hashPassword("made-up-secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	ok, err := verifyPassword(stored, "made-up-secret")
	if err != nil || !ok {
		t.Fatalf("verifyPassword(own hash, same password) = %v, %v; want true, nil", ok, err)
	}
}

func TestHashFormatIsPHC(t *testing.T) {
	t.Parallel()
	stored, err := hashPassword("made-up-secret")
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
	first, err := hashPassword("made-up-secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	second, err := hashPassword("made-up-secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if first == second {
		t.Fatalf("two hashes of the same password are equal (%q); want a random salt each time", first)
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	t.Parallel()
	stored, err := hashPassword("made-up-secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	ok, err := verifyPassword(stored, "made-up-secreT")
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
			ok, err := verifyPassword(tt.stored, "made-up-secret")
			if err == nil || ok {
				t.Fatalf("verifyPassword(%q) = %v, %v; want false and an error", tt.stored, ok, err)
			}
		})
	}
}

func TestVerifyHonoursStoredParams(t *testing.T) {
	t.Parallel()
	stored, err := hashWith("made-up-secret", argonParams{memory: 65536, iterations: 1, threads: 1})
	if err != nil {
		t.Fatalf("hashWith: %v", err)
	}
	if !strings.HasPrefix(stored, "$argon2id$v=19$m=65536,t=1,p=1$") {
		t.Fatalf("hashWith(t=1) = %q; want t=1 in the parameters", stored)
	}
	ok, err := verifyPassword(stored, "made-up-secret")
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
