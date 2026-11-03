package clock

import (
	"testing"
	"time"
)

func TestFake(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	f := NewFake(start)
	if got := f.Now(); !got.Equal(start) {
		t.Fatalf("Now() = %v, want %v", got, start)
	}
	f.Advance(90 * time.Minute)
	if got, want := f.Now(), start.Add(90*time.Minute); !got.Equal(want) {
		t.Fatalf("after Advance, Now() = %v, want %v", got, want)
	}
	later := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	f.Set(later)
	if got := f.Now(); !got.Equal(later) {
		t.Fatalf("after Set, Now() = %v, want %v", got, later)
	}
}
