// Package basic implements the basic EigenTrust algorithm.
package basic

import (
	"context"

	"github.com/pkg/errors"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

// ErrZeroSum signals that an input vector's components sum to zero.
var ErrZeroSum = errors.New("zero sum")

// ErrDimensionMismatch signals a dimension mismatch
// between related data structures,
// ex: a local trust matrix and a pre-trust vector.
var ErrDimensionMismatch = errors.New("dimension mismatch")

// CsvReader reads from a CSV file.
type CsvReader interface {
	Read() (fields []string, err error)
}

// Canonicalize scales input so that their components sum to one.
//
// If input sums to zero, Canonicalize returns ErrZeroSum.
//
// If output is not nil, Canonicalize uses it (after size checking).
// Otherwise, Canonicalize allocates a new slice to use.
// In both cases, the slice is returned upon success.
func Canonicalize(input []float64, output []float64) ([]float64, error) {
	s := floats.SumCompensated(input)
	if s == 0 {
		return nil, ErrZeroSum
	}
	if output == nil {
		output = make([]float64, len(input))
	} else if len(input) != len(output) {
		return nil, ErrDimensionMismatch
	}
	inVec := mat.NewVecDense(len(input), input)
	outVec := mat.NewVecDense(len(output), output)
	outVec.ScaleVec(1/s, inVec) // places result in output
	return output, nil
}

// Compute computes EigenTrust scores.
//
// Local trust (c) and pre-trust (p) must have already been canonicalized.
//
// Compute uses p as the starting point for iterations,
// except if t0 (initial trust vector) is not nil,
// Compute uses t0 instead of p.
//
// Compute stores the result (EigenTrust scores) in t and returns it.
// If t is nil, Compute allocates one first and returns it.
func Compute(
	ctx context.Context, c LocalTrust, p TrustVector, a float64, e float64,
	t0 TrustVector, t TrustVector,
) (TrustVector, error) {
	n := c.Dim()
	if n == 0 {
		return nil, errors.New("empty local trust")
	}
	if p.Len() != n ||
		(t0 != nil && t0.Len() != n) ||
		(t != nil && t.Len() != n) {
		return nil, ErrDimensionMismatch
	}
	if a < 0 || a > 1 {
		return nil, errors.Errorf("hunch %#v out of range [0..1]", a)
	}
	if e <= 0 {
		return nil, errors.Errorf("epsilon %#v is not positive", e)
	}

	if t0 == nil {
		t0 = p
	}
	t1c := append([]float64(nil), t0.Components()...)
	t1 := mat.NewVecDense(n, t1c)

	d := 2 * e
	ct := c.T()
	ap := &mat.VecDense{}
	ap.ScaleVec(a, p)
	for d > e {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		x := mat.VecDenseCopyOf(t1)
		t1.MulVec(ct, t1)
		t1.AddScaledVec(ap, 1-a, t1)
		x.SubVec(t1, x) // new t1 - old t1
		d = x.Norm(2)
	}
	if t == nil {
		t = NewTrustVector(n, t1c)
	} else {
		copy(t.Components(), t1c)
	}
	return t, nil
}
