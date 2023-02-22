// Package basic implements the basic EigenTrust algorithm.
package basic

import (
	"context"
	"math"
	"time"

	"github.com/pkg/errors"
	"k3l.io/go-eigentrust/pkg/sparse"
)

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
		return sparse.ErrZeroSum
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
	logger, hasLogger := LoggerInContext(ctx)
	if hasLogger {
		logger.Trace().Msg("started")
	}
	tm0 := time.Now()
	n, err := c.Dim()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errors.New("empty local trust")
	}
	if p.Dim != n ||
		(t0 != nil && t0.Dim != n) ||
		(t != nil && t.Dim != n) {
		return nil, sparse.ErrDimensionMismatch
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

	d0 := 1.0
	d := 2 * e // initial sentinel
	ct, err := c.Transpose(ctx)
	if err != nil {
		return nil, err
	}
	ap := &sparse.Vector{}
	ap.ScaleVec(a, p)
	tm1 := time.Now()
	durPrep, tm0 := tm1.Sub(tm0), tm1
	iter := 0
	for ; d > e; iter++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		t1Old := t1.Clone()
		err = t1.MulVec(ctx, ct, t1)
		if err != nil {
			return nil, err
		}
		t1.ScaleVec(1-a, t1)
		err = t1.AddVec(t1, ap)
		if err != nil {
			return nil, err
		}
		err = t1Old.SubVec(t1, t1Old)
		if err != nil {
			return nil, err
		}
		d = t1Old.Norm2()
		if hasLogger {
			logger.Trace().
				Int("iteration", iter).
				Float64("log10dPace", math.Log10(d/e)).
				Float64("log10dRemaining", math.Log10(d/d0)).
				Msg("one iteration")
		}
		d0 = d
	}
	tm1 = time.Now()
	durIter, tm0 := tm1.Sub(tm0), tm1
	if hasLogger {
		logger.Debug().
			Int("dim", n).
			Int("nnz", ct.NNZ()).
			Float64("alpha", a).
			Float64("epsilon", e).
			Int("iterations", iter).
			Dur("durPrep", durPrep).
			Dur("durIter", durIter).
			Msg("finished")
	}
	if t == nil {
		t = t1
	} else {
		t.Assign(t1)
	}
	return t, nil
}
