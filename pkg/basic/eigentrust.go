// Package basic implements the basic EigenTrust algorithm.
package basic

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"time"

	"github.com/rs/zerolog"
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
// Alpha (a) and epsilon (e) are the pre-trust bias and iteration threshold,
// as defined in the EigenTrust paper.
//
// Compute accepts options (opts) which modifies its behavior.
// See their documentation for details.
//
// Compute terminates EigenTrust iterations when the trust vector converges,
// i.e. the Frobenius norm of trust vector delta falls below epsilon threshold.
// The convergence check is done by default every iteration;
// WithMaxIterations, WithMinIterations, and WithCheckFreq changes the timing.
//
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
	logger := zerolog.Ctx(ctx)
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
		return nil, fmt.Errorf("hunch %#v out of range [0..1]", a)
	}
	if e <= 0 {
		return nil, fmt.Errorf("epsilon %#v is not positive", e)
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
	checkFreq := 1
	if o.checkFreq != nil {
		checkFreq = *o.checkFreq
	}
	if checkFreq < 1 {
		return nil, fmt.Errorf("checkFreq=%d must be positive", checkFreq)
	}
	maxIters := 0
	if o.maxIterations != nil {
		maxIters = *o.maxIterations
	}
	if maxIters < 0 {
		return nil, fmt.Errorf(
			"maxIters=%d must be either 0 (unlimited) or positive", maxIters)
	}
	if maxIters == 0 {
		maxIters = math.MaxInt
	}
	minIters := checkFreq
	if o.minIterations != nil {
		minIters = *o.minIterations
	}
	if minIters <= 0 {
		return nil, fmt.Errorf("minIters=%d must be at least 1", minIters)
	}
	// hard-cap at maxIters
	iter := 0
	for ; iter < maxIters; iter++ {
		// check exit criteria,
		// first at minIters then every checkFreq iterations afterward.
		if iter >= minIters && (iter-minIters)%checkFreq == 0 {
			converged := d <= e
			flatTailReached := flatTailStats.Length >= flatTail
			if converged && flatTailReached {
				// both criteria met
				break
			}
		}
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
		logger.Trace().
			Int("iteration", iter).
			Float64("log10dPace", math.Log10(d/d0)).
			Float64("log10dRemaining", math.Log10(d/e)).
			Msg("one iteration")
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
			if flatTailStats.Length > 0 {
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
	if t == nil {
		t = t1
	} else {
		t.Assign(t1)
	}
	return t, nil
}

// DiscountTrustVector adjusts the given global trust vector
// by the negative trust given in the discounts vector.
//
// DiscountTrustVector does this
// by scaling non-zero discount rows with the distruster's own trust score
// and subtracting the scaled discount row from the global trust vector.
//
// The caller shall ensure that the discounts vector is canonicalized.
func DiscountTrustVector(t *sparse.Vector, discounts *sparse.Matrix) error {
	// t is adjusted in place, so take the unadjusted clone for discount weight.
	i1 := 0
	t1 := t.Clone()
	// find distrusters with nonzero reps in t1 by merge-matching
DiscountsLoop:
	for distruster, distrusts := range discounts.Entries {
	T1Loop:
		for {
			switch {
			case i1 >= len(t1.Entries):
				// no more nonzero trust, remaining distrusters have zero rep
				// and their distrusts do not matter, so finish
				break DiscountsLoop
			case t1.Entries[i1].Index < distruster:
				// the peer at i1 has no distrust,
				// advance to the next peer
				i1++
				continue T1Loop
			case t1.Entries[i1].Index == distruster:
				// found a match!
				break T1Loop
			case t1.Entries[i1].Index > distruster:
				// distruster has zero rep,
				// advance to the next distruster
				continue DiscountsLoop
			}
		}
		scaledDistrustVec := &sparse.Vector{}
		scaledDistrustVec.ScaleVec(t1.Entries[i1].Value, &sparse.Vector{
			Dim:     t.Dim,
			Entries: distrusts,
		})
		if err := t.SubVec(t, scaledDistrustVec); err != nil {
			return err
		}
		i1++
	}
	return nil
}
