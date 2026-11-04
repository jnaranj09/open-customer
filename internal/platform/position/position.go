// Package position holds the one rule for the "position" column of the ordered catalog tables
// (attributes, options, services and the modules that follow): 0 up to Max.
package position

import "math"

// Max is the largest position a request may set. It leaves room for Next to add one without
// passing the int32 column.
const Max = math.MaxInt32 - 1

// Check reports whether n is a position (0 to Max) and returns it as the column type.
func Check(n int) (int32, bool) {
	if n < 0 || n > Max {
		return 0, false
	}
	return int32(n), true
}

// Next is the position after last, the largest stored one (-1 when there is none). It reports
// false when last is already Max, so a create never wraps the column.
func Next(last int32) (int32, bool) {
	if last >= Max {
		return 0, false
	}
	return last + 1, true
}
