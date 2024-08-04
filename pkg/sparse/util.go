package sparse

import (
	"math"
)

// NilIfEmpty returns the given slice, except if empty, it returns nil.
func NilIfEmpty[T any](slice []T) []T {
	if len(slice) == 0 {
		return nil
	}
	return slice
}

// Filter returns the slice elements for which the predicate evaluates true.
func Filter[T any](slice []T, pred func(T) bool) []T {
	var filtered []T
	for _, element := range slice {
		if pred(element) {
			filtered = append(filtered, element)
		}
	}
	return filtered
}

// KBNSummer is the Kahan-Babushka-Neumaier compensated summation algorithm.
type KBNSummer struct {
	sum, compensation float64
}

func (s *KBNSummer) Add(value float64) {
	moreSig, lessSig := s.sum, value
	if math.Abs(moreSig) < math.Abs(lessSig) {
		moreSig, lessSig = lessSig, moreSig
	}
	s.sum += value
	// During summation above, essentially moreSig + lessSig,
	// lessSig's exponent were brought up to match moreSig's,
	// so lessSig had low-order bits truncated.
	// Recover this "truncated lessSig" used in the addition.
	truncatedLessSig := s.sum - moreSig
	// Now lessSig and truncatedLessSig should be back on the same
	// exponent scale; the difference is the truncated bits (error).
	s.compensation += lessSig - truncatedLessSig

	// If you got here looking for the root cause
	// of still inaccurate summation:
	// `compensation` above itself is a sum,
	// so it will likely accumulate its own error.
	// That error can in turn be compensated further by introducing
	// a second-order compensation, that is,
	// "compensation of errors in `compensation` summation".
	// In general, we can repeat this for even higher orders
	// until we achieve desired accuracy
	// (generalized Kahan-Babushka summation, aka Kahan-Babushka-Klein).
}

func (s *KBNSummer) Sum() float64 {
	return s.sum + s.compensation
}
