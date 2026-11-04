package position

import (
	"math"
	"testing"
)

func TestCheckAccepts(t *testing.T) {
	for _, tc := range []struct {
		name string
		n    int
	}{
		{"zero", 0},
		{"one", 1},
		{"Max", Max},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Check(tc.n)
			if !ok || int(got) != tc.n {
				t.Fatalf("Check(%d) = %d, %v, want %d, true", tc.n, got, ok, tc.n)
			}
		})
	}
}

func TestCheckRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		n    int
	}{
		{"minus one", -1},
		{"Max plus one", Max + 1},
		{"beyond int32", math.MaxInt32 + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := Check(tc.n); ok {
				t.Fatalf("Check(%d) = %d, true, want false", tc.n, got)
			}
		})
	}
}

func TestNextAdvances(t *testing.T) {
	for _, tc := range []struct {
		name       string
		last, want int32
	}{
		{"empty table", -1, 0},
		{"after zero", 0, 1},
		{"after Max-1", Max - 1, Max},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Next(tc.last)
			if !ok || got != tc.want {
				t.Fatalf("Next(%d) = %d, %v, want %d, true", tc.last, got, ok, tc.want)
			}
		})
	}
}

func TestNextRefusesAtMax(t *testing.T) {
	for _, last := range []int32{Max, math.MaxInt32} {
		if got, ok := Next(last); ok {
			t.Fatalf("Next(%d) = %d, true, want false", last, got)
		}
	}
}
