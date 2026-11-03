// Package ids creates and checks record IDs (UUIDv7).
package ids

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrInvalid is returned by Parse for any ID that is not a canonical UUID.
var ErrInvalid = errors.New("invalid id")

// New returns a new UUIDv7.
func New() (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("new uuidv7: %w", err)
	}
	return id, nil
}

// Parse accepts only the canonical 36-character form, so the same ID never has two spellings.
func Parse(s string) (uuid.UUID, error) {
	if len(s) != 36 {
		return uuid.Nil, ErrInvalid
	}
	id, err := uuid.Parse(s)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, ErrInvalid
	}
	return id, nil
}
