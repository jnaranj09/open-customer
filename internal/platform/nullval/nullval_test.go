package nullval

import (
	"testing"

	"github.com/oapi-codegen/nullable"
)

func TestPtr(t *testing.T) {
	if got := Ptr(nullable.NewNullableWithValue("Lucía")); got == nil || *got != "Lucía" {
		t.Errorf("value: got %v, want Lucía", got)
	}
	if got := Ptr(nullable.NewNullNullable[int64]()); got != nil {
		t.Errorf("null: got %d, want nil", *got)
	}
	if got := Ptr(nullable.Nullable[int64]{}); got != nil {
		t.Errorf("unspecified: got %d, want nil", *got)
	}
}

func TestOf(t *testing.T) {
	n := int64(2500)
	if got := Of(&n); !got.IsSpecified() || got.IsNull() || got.MustGet() != 2500 {
		t.Errorf("value: got %v, want 2500", got)
	}
	if got := Of[string](nil); !got.IsSpecified() || !got.IsNull() {
		t.Errorf("nil: got %v, want an explicit null", got)
	}
}
