package ids

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewIsV7(t *testing.T) {
	t.Parallel()
	id, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if id.Version() != 7 {
		t.Fatalf("version = %d, want 7", id.Version())
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	valid := "01890a5d-ac96-774b-bcce-b302099a8057"
	got, err := Parse(valid)
	if err != nil || got.String() != valid {
		t.Fatalf("Parse(%q) = %v, %v; want the same id", valid, got, err)
	}
	for _, bad := range []string{
		"",
		"not-a-uuid",
		"{01890a5d-ac96-774b-bcce-b302099a8057}",
		"urn:uuid:01890a5d-ac96-774b-bcce-b302099a8057",
		"01890a5dac96774bbcceb302099a8057",
		uuid.Nil.String(),
	} {
		if _, err := Parse(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("Parse(%q) error = %v, want ErrInvalid", bad, err)
		}
	}
}
