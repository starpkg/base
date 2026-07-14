package util

import (
	"math"
	"time"
)

// DurationFromSeconds converts a fractional number of seconds to a time.Duration
// without the truncation bug of time.Duration(sec) * time.Second, which converts
// the float to an integer BEFORE multiplying and so silently drops any
// sub-second part (0.5 -> 0, an immediately-firing deadline). It multiplies in
// float space first, then clamps to the representable range instead of
// overflowing/wrapping; a NaN yields 0.
func DurationFromSeconds(sec float64) time.Duration {
	if math.IsNaN(sec) {
		return 0
	}
	ns := sec * float64(time.Second)
	switch {
	case ns >= float64(math.MaxInt64):
		return time.Duration(math.MaxInt64)
	case ns <= float64(math.MinInt64):
		return time.Duration(math.MinInt64)
	default:
		return time.Duration(ns)
	}
}
