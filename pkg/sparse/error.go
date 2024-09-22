package sparse

import (
	"errors"
	"fmt"
)

// ErrZeroSum signals that an input vector's components sum to zero.
var ErrZeroSum = errors.New("zero sum")

// ErrDimensionMismatch signals a dimension mismatch
// between related data structures,
// ex: a local trust matrix and a pre-trust vector.
var ErrDimensionMismatch = errors.New("dimension mismatch")

// NegativeValueError signals a negative-valued entry was encountered
// where disallowed.
type NegativeValueError struct {
	Value float64
}

func (e NegativeValueError) Error() string {
	return fmt.Sprintf("negative value %#v not allowed", e.Value)
}
