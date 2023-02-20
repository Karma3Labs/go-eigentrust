// Package basic implements the basic EigenTrust algorithm.
package basic

import (
	"context"

	"github.com/pkg/errors"
	"k3l.io/go-eigentrust/pkg/sparse"
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

// Canonicalize scales sparse entries in-place so that their values sum to one.
//
// If entries sum to zero, Canonicalize returns ErrZeroSum.
func Canonicalize(entries []sparse.Entry) error {
	var summer sparse.KBNSummer
	for _, entry := range entries {
		summer.Add(entry.Value)
	}
	s := summer.Sum()
	if s == 0 {
		return ErrZeroSum
	}
	for i := range entries {
		entries[i].Value /= s
	}
	return nil
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
	ctx context.Context, c *sparse.Matrix, p *sparse.Vector, a float64,
	e float64,
	t0 *sparse.Vector, t *sparse.Vector,
) (*sparse.Vector, error) {
	if c.Rows() != c.Columns() {
		return nil, errors.Errorf("local trust is not a square matrix (%dx%d)",
			c.Rows(), c.Columns())
	}
	n := c.Rows()
	if n == 0 {
		return nil, errors.New("empty local trust")
	}
	if p.Dim != n ||
		(t0 != nil && t0.Dim != n) ||
		(t != nil && t.Dim != n) {
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
	t1 := t0.Clone()

	d := 2 * e // initial sentinel
	ct := c.Transpose()
	ap := &sparse.Vector{}
	ap.ScaleVec(a, p)
	for d > e {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		t1Old := t1.Clone()
		t1.MulVec(ct, t1)
		t1.ScaleVec(1-a, t1)
		t1.AddVec(t1, ap)
		t1Old.SubVec(t1, t1Old)
		d = t1Old.Norm2()
	}
	if t == nil {
		t = t1
	} else {
		t.Assign(t1)
	}
	return t, nil
}
