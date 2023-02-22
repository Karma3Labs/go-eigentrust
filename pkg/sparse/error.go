package sparse

import "github.com/pkg/errors"

// ErrZeroSum signals that an input vector's components sum to zero.
var ErrZeroSum = errors.New("zero sum")

// ErrDimensionMismatch signals a dimension mismatch
// between related data structures,
// ex: a local trust matrix and a pre-trust vector.
var ErrDimensionMismatch = errors.New("dimension mismatch")
