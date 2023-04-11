package basic

import "C"
import (
	"context"
	"fmt"
	"runtime"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"k3l.io/go-eigentrust/pkg/sparse"
	"k3l.io/go-eigentrust/pkg/util"
)

type StrictServerImpl struct {
	Logger zerolog.Logger
}

type Error400 struct {
	Inner error
}

func (e Error400) Error() string {
	return fmt.Sprintf("400 Bad Request: %s", e.Inner.Error())
}

func (server *StrictServerImpl) compute(
	ctx context.Context, localTrustRef *LocalTrustRef, preTrust *TrustVectorRef,
	alpha *float64, epsilon *float64,
	flatTail *int, numLeaders *int,
) (tv TrustVectorRef, flatTailStats FlatTailStats, err error) {
	logger := server.Logger.With().
		Str("func", "(*StrictServerImpl).Compute").
		Logger()
	var (
		c *sparse.Matrix
		p *sparse.Vector
	)
	if c, err = server.loadLocalTrust(localTrustRef); err != nil {
		err = Error400{errors.Wrap(err, "cannot load local trust")}
		return
	}
	cDim, err := c.Dim()
	if err != nil {
		return
	}
	logger.Trace().
		Int("dim", cDim).
		Int("nnz", c.NNZ()).
		Msg("local trust loaded")
	if preTrust == nil {
		// Default to zero pre-trust (canonicalized into uniform later).
		p = sparse.NewVector(cDim, nil)
	} else if p, err = loadTrustVector(preTrust); err != nil {
		err = Error400{errors.Wrap(err, "cannot load pre-trust")}
		return
	} else {
		// align dimensions
		switch {
		case p.Dim < cDim:
			p.SetDim(cDim)
		case cDim < p.Dim:
			cDim = p.Dim
			c.SetDim(p.Dim, p.Dim)
		}
	}
	logger.Trace().
		Int("dim", p.Dim).
		Int("nnz", p.NNZ()).
		Msg("pre-trust loaded")
	if alpha == nil {
		a := 0.5
		alpha = &a
	} else if *alpha < 0 || *alpha > 1 {
		err = Error400{errors.Errorf("alpha=%f out of range [0..1]", *alpha)}
		return
	}
	if epsilon == nil {
		e := 1e-6 / float64(cDim)
		epsilon = &e
	} else if *epsilon <= 0 || *epsilon > 1 {
		err = Error400{
			errors.Errorf("epsilon=%f out of range (0..1]", *epsilon),
		}
		return
	}
	if flatTail == nil {
		ft := 0
		flatTail = &ft
	}
	if numLeaders == nil {
		nl := 0
		numLeaders = &nl
	}
	CanonicalizeTrustVector(p)
	err = CanonicalizeLocalTrust(c, p)
	if err != nil {
		err = Error400{errors.Wrapf(err, "cannot canonicalize local trust")}
		return
	}
	t, err := Compute(ctx, c, p, *alpha, *epsilon, nil, nil,
		*flatTail, *numLeaders, &flatTailStats)
	c = nil
	p = nil
	runtime.GC()
	if err != nil {
		err = errors.Wrapf(err, "cannot compute EigenTrust")
		return
	}
	var itv InlineTrustVector
	itv.Scheme = "inline" // FIXME(ek): can we not hard-code this?
	itv.Size = t.Dim
	for _, e := range t.Entries {
		itv.Entries = append(itv.Entries,
			InlineTrustVectorEntry{I: e.Index, V: e.Value})
	}
	if err = tv.FromInlineTrustVector(itv); err != nil {
		err = errors.Wrapf(err, "cannot create response")
		return
	}
	return tv, flatTailStats, nil
}

func (server *StrictServerImpl) Compute(
	ctx context.Context, request ComputeRequestObject,
) (ComputeResponseObject, error) {
	ctx = util.SetLoggerInContext(ctx, server.Logger)
	tv, _, err := server.compute(ctx,
		&request.Body.LocalTrust, request.Body.PreTrust,
		request.Body.Alpha, request.Body.Epsilon,
		request.Body.FlatTail, request.Body.NumLeaders)
	if err != nil {
		if error400, ok := err.(Error400); ok {
			var resp Compute400JSONResponse
			resp.Message = error400.Inner.Error()
			return resp, nil
		}
		return nil, err
	}
	return Compute200JSONResponse(tv), nil
}

func (server *StrictServerImpl) ComputeWithStats(
	ctx context.Context, request ComputeWithStatsRequestObject,
) (ComputeWithStatsResponseObject, error) {
	ctx = util.SetLoggerInContext(ctx, server.Logger)
	tv, flatTailStats, err := server.compute(ctx,
		&request.Body.LocalTrust, request.Body.PreTrust,
		request.Body.Alpha, request.Body.Epsilon,
		request.Body.FlatTail, request.Body.NumLeaders)
	if err != nil {
		if error400, ok := err.(Error400); ok {
			var resp ComputeWithStats400JSONResponse
			resp.Message = error400.Inner.Error()
			return resp, nil
		}
		return nil, err
	}
	var resp ComputeWithStats200JSONResponse
	resp.EigenTrust = tv
	resp.FlatTailStats.Length = flatTailStats.Length
	resp.FlatTailStats.Threshold = flatTailStats.Threshold
	resp.FlatTailStats.DeltaNorm = flatTailStats.DeltaNorm
	resp.FlatTailStats.Ranking = flatTailStats.Ranking
	return resp, nil
}

func (server *StrictServerImpl) loadLocalTrust(
	ref *LocalTrustRef,
) (*sparse.Matrix, error) {
	if inline, err := ref.AsInlineLocalTrust(); err == nil {
		return server.loadInlineLocalTrust(&inline)
	}
	return nil, errors.New("unknown local trust ref type")
}

func (server *StrictServerImpl) loadInlineLocalTrust(
	inline *InlineLocalTrust,
) (*sparse.Matrix, error) {
	if inline.Size <= 0 {
		return nil, errors.Errorf("invalid size=%#v", inline.Size)
	}
	var entries []sparse.CooEntry
	for idx, entry := range inline.Entries {
		if entry.I < 0 || entry.I >= inline.Size {
			return nil, errors.Errorf("entry %d: i=%d is out of range [0..%d)",
				idx, entry.I, inline.Size)
		}
		if entry.J < 0 || entry.J >= inline.Size {
			return nil, errors.Errorf("entry %d: j=%d is out of range [0..%d)",
				idx, entry.J, inline.Size)
		}
		if entry.V <= 0 {
			return nil, errors.Errorf("entry %d: v=%f is out of range (0, inf)",
				idx, entry.V)
		}
		entries = append(entries, sparse.CooEntry{
			Row:    entry.I,
			Column: entry.J,
			Value:  entry.V,
		})
	}
	// reset after move
	size := inline.Size
	inline.Size = 0
	inline.Entries = nil
	return sparse.NewCSRMatrix(size, size, entries), nil
}

func loadTrustVector(ref *TrustVectorRef) (*sparse.Vector, error) {
	if inline, err := ref.AsInlineTrustVector(); err == nil {
		return loadInlineTrustVector(&inline)
	}
	return nil, errors.New("unknown trust vector type")
}

func loadInlineTrustVector(inline *InlineTrustVector) (*sparse.Vector, error) {
	var entries []sparse.Entry
	for idx, entry := range inline.Entries {
		if entry.I < 0 || entry.I >= inline.Size {
			return nil, errors.Errorf("entry %d: i=%d is out of range [0..%d)",
				idx, entry.I, inline.Size)
		}
		if entry.V <= 0 {
			return nil, errors.Errorf("entry %d: v=%f is out of range (0, inf)",
				idx, entry.V)
		}
		entries = append(entries, sparse.Entry{Index: entry.I, Value: entry.V})
	}
	// reset after move
	size := inline.Size
	inline.Size = 0
	inline.Entries = nil
	return sparse.NewVector(size, entries), nil
}
