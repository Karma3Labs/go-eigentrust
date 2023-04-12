// Package basic implements the basic EigenTrust algorithm.
package basic

import (
	"context"
	"math"
	"reflect"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"k3l.io/go-eigentrust/pkg/sparse"
	"k3l.io/go-eigentrust/pkg/util"
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
// Alpha (a) and epsilon (e) are the pre-trust bias and iteration threshold,
// as defined in the EigenTrust paper.
//
// Compute accepts options (opts) which modifies its behavior.
// See their documentation for details.
//
// Compute terminates EigenTrust iterations when the trust vector converges,
// i.e. the Frobenius norm of trust vector delta falls below epsilon threshold.
// Also see WithFlatTail for an additional/alternative termination criterion
// based upon ranking stability.
func Compute(
	ctx context.Context, c *sparse.Matrix, p *sparse.Vector,
	a float64, e float64,
	opts ...ComputeOpt,
) (*sparse.Vector, error) {
	o := ComputeOpts{}
	for _, opt := range opts {
		opt(&o)
	}
	t0 := o.t0
	t := o.t
	flatTail := o.flatTailLength
	numLeaders := o.numLeaders
	flatTailStats := o.flatTailStats
	logger, hasLogger := util.LoggerInContext(ctx)
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
	if numLeaders == 0 {
		numLeaders = n
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
	if flatTailStats == nil {
		flatTailStats = &FlatTailStats{}
	}
	flatTailStats.Length = 0
	flatTailStats.Threshold = 1
	flatTailStats.DeltaNorm = 1
	flatTailStats.Ranking = nil
	iter := 0
	for ; d > e || (flatTailStats.Length < flatTail); iter++ {
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
				Float64("log10dPace", math.Log10(d/d0)).
				Float64("log10dRemaining", math.Log10(d/e)).
				Msg("one iteration")
		}
		d0 = d
		entries := sparse.SortEntriesByValue(
			append(t1.Entries[:0:0], t1.Entries...))
		ranking := make([]int, 0, len(entries))
		for _, entry := range entries {
			ranking = append(ranking, entry.Index)
		}
		if len(ranking) > numLeaders {
			ranking = ranking[:numLeaders]
		}
		if reflect.DeepEqual(ranking, flatTailStats.Ranking) {
			flatTailStats.Length++
		} else {
			if hasLogger && flatTailStats.Length > 0 {
				logger.Trace().
					Int("length", flatTailStats.Length).
					Msg("false flat tail detected")
			}
			if flatTailStats.Threshold <= flatTailStats.Length {
				flatTailStats.Threshold = flatTailStats.Length + 1
			}
			flatTailStats.Length = 0
			flatTailStats.DeltaNorm = d
			flatTailStats.Ranking = ranking
		}
		runtime.GC()
	}
	tm1 = time.Now()
	durIter, tm0 := tm1.Sub(tm0), tm1
	if hasLogger {
		logger.Debug().
			Int("dim", n).
			Int("nnz", ct.NNZ()).
			Float64("alpha", a).
			Float64("epsilon", e).
			Int("flatTail", flatTail).
			Int("numLeaders", numLeaders).
			Int("iterations", iter).
			Dur("durPrep", durPrep).
			Dur("durIter", durIter).
			Msg("finished")
		logger.Trace().
			Int("length", flatTailStats.Length).
			Int("threshold", flatTailStats.Threshold).
			Float64("deltaNorm", flatTailStats.DeltaNorm).
			Msg("flat tail stats")
	}
	if t == nil {
		t = t1
	} else {
		t.Assign(t1)
	}
	return t, nil
}
