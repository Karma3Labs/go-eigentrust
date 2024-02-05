package oapiserver

import "C"

import (
	"context"
	"math/big"
	"runtime"

	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"k3l.io/go-eigentrust/pkg/api/openapi"
	"k3l.io/go-eigentrust/pkg/basic"
	"k3l.io/go-eigentrust/pkg/basic/server"
	"k3l.io/go-eigentrust/pkg/sparse"
)

type StrictServerImpl struct {
	core *server.Core
}

func NewStrictServerImpl(logger zerolog.Logger) *StrictServerImpl {
	return &StrictServerImpl{
		core: server.NewCore(logger),
	}
}

func (svr *StrictServerImpl) compute(
	ctx context.Context, localTrustRef *openapi.LocalTrustRef,
	initialTrust *openapi.TrustVectorRef, preTrust *openapi.TrustVectorRef,
	alpha *float64, epsilon *float64,
	flatTail *int, numLeaders *int,
	maxIterations *int,
) (tv openapi.TrustVectorRef, flatTailStats basic.FlatTailStats, err error) {
	logger := svr.core.Logger.With().
		Str("func", "(*StrictServerImpl).Compute").
		Logger()
	var (
		c  *sparse.Matrix
		p  *sparse.Vector
		t0 *sparse.Vector
	)
	opts := []basic.ComputeOpt{basic.WithFlatTailStats(&flatTailStats)}
	if c, err = svr.loadLocalTrust(localTrustRef); err != nil {
		err = server.HTTPError{
			Code: 400, Inner: errors.Wrap(err, "cannot load local trust"),
		}
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
		err = server.HTTPError{
			Code: 400, Inner: errors.Wrap(err, "cannot load pre-trust"),
		}
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
	if initialTrust == nil {
		t0 = nil
	} else if t0, err = loadTrustVector(initialTrust); err != nil {
		err = server.HTTPError{
			Code: 400, Inner: errors.Wrap(err, "cannot load initial trust"),
		}
		return
	} else {
		// align dimensions
		switch {
		case t0.Dim < cDim:
			t0.SetDim(cDim)
		case cDim < t0.Dim:
			cDim = t0.Dim
			c.SetDim(t0.Dim, t0.Dim)
			p.SetDim(t0.Dim)
		}
		logger.Trace().
			Int("dim", t0.Dim).
			Int("nnz", t0.NNZ()).
			Msg("initial trust loaded")
		opts = append(opts, basic.WithInitialTrust(t0))
	}
	if alpha == nil {
		a := 0.5
		alpha = &a
	} else if *alpha < 0 || *alpha > 1 {
		err = server.HTTPError{
			Code:  400,
			Inner: errors.Errorf("alpha=%f out of range [0..1]", *alpha),
		}
		return
	}
	if epsilon == nil {
		e := 1e-6 / float64(cDim)
		epsilon = &e
	} else if *epsilon <= 0 || *epsilon > 1 {
		err = server.HTTPError{
			Code:  400,
			Inner: errors.Errorf("epsilon=%f out of range (0..1]", *epsilon),
		}
		return
	}
	if flatTail != nil {
		opts = append(opts, basic.WithFlatTail(*flatTail))
	}
	if numLeaders != nil {
		opts = append(opts, basic.WithFlatTailNumLeaders(*numLeaders))
	}
	if maxIterations != nil {
		opts = append(opts, basic.WithMaxIterations(*maxIterations))
	}
	basic.CanonicalizeTrustVector(p)
	if t0 != nil {
		basic.CanonicalizeTrustVector(t0)
	}
	discounts, err := basic.ExtractDistrust(c)
	if err != nil {
		err = server.HTTPError{
			Code: 400, Inner: errors.Wrapf(err, "cannot extract discounts"),
		}
		return
	}
	err = basic.CanonicalizeLocalTrust(c, p)
	if err != nil {
		err = server.HTTPError{
			Code:  400,
			Inner: errors.Wrapf(err, "cannot canonicalize local trust"),
		}
		return
	}
	err = basic.CanonicalizeLocalTrust(discounts, nil)
	if err != nil {
		err = server.HTTPError{
			Code:  400,
			Inner: errors.Wrapf(err, "cannot canonicalize discounts"),
		}
		return
	}
	t, err := basic.Compute(ctx, c, p, *alpha, *epsilon, opts...)
	c = nil
	p = nil
	runtime.GC()
	if err != nil {
		err = errors.Wrapf(err, "cannot compute EigenTrust")
		return
	}
	basic.DiscountTrustVector(t, discounts)
	var itv openapi.InlineTrustVector
	itv.Scheme = "inline" // FIXME(ek): can we not hard-code this?
	itv.Size = t.Dim
	for _, e := range t.Entries {
		itv.Entries = append(itv.Entries,
			openapi.InlineTrustVectorEntry{I: e.Index, V: e.Value})
	}
	if err = tv.FromInlineTrustVector(itv); err != nil {
		err = errors.Wrapf(err, "cannot create response")
		return
	}
	return tv, flatTailStats, nil
}

func (svr *StrictServerImpl) Compute(
	ctx context.Context, request openapi.ComputeRequestObject,
) (openapi.ComputeResponseObject, error) {
	ctx = svr.core.Logger.WithContext(ctx)
	tv, _, err := svr.compute(ctx,
		&request.Body.LocalTrust,
		request.Body.InitialTrust, request.Body.PreTrust,
		request.Body.Alpha, request.Body.Epsilon,
		request.Body.FlatTail, request.Body.NumLeaders,
		request.Body.MaxIterations)
	if err != nil {
		var httpError server.HTTPError
		if errors.As(err, &httpError) {
			switch httpError.Code {
			case 400:
				var resp openapi.Compute400JSONResponse
				resp.Message = httpError.Inner.Error()
				return resp, nil
			}
		}
		return nil, err
	}
	return openapi.Compute200JSONResponse(tv), nil
}

func (svr *StrictServerImpl) ComputeWithStats(
	ctx context.Context, request openapi.ComputeWithStatsRequestObject,
) (openapi.ComputeWithStatsResponseObject, error) {
	ctx = svr.core.Logger.WithContext(ctx)
	tv, flatTailStats, err := svr.compute(ctx,
		&request.Body.LocalTrust,
		request.Body.InitialTrust, request.Body.PreTrust,
		request.Body.Alpha, request.Body.Epsilon,
		request.Body.FlatTail, request.Body.NumLeaders,
		request.Body.MaxIterations)
	if err != nil {
		var httpError server.HTTPError
		if errors.As(err, &httpError) {
			switch httpError.Code {
			case 400:
				var resp openapi.ComputeWithStats400JSONResponse
				resp.Message = httpError.Inner.Error()
				return resp, nil
			}
		}
		return nil, err
	}
	var resp openapi.ComputeWithStats200JSONResponse
	resp.EigenTrust = tv
	resp.FlatTailStats.Length = flatTailStats.Length
	resp.FlatTailStats.Threshold = flatTailStats.Threshold
	resp.FlatTailStats.DeltaNorm = flatTailStats.DeltaNorm
	resp.FlatTailStats.Ranking = flatTailStats.Ranking
	return resp, nil
}

func (svr *StrictServerImpl) GetLocalTrust(
	ctx context.Context, request openapi.GetLocalTrustRequestObject,
) (openapi.GetLocalTrustResponseObject, error) {
	inline, err := svr.getLocalTrust(ctx, request.Id)
	if err != nil {
		var httpError server.HTTPError
		if errors.As(err, &httpError) {
			switch httpError.Code {
			case 404:
				return openapi.GetLocalTrust404Response{}, nil
			}
		}
		return nil, err
	}
	return openapi.GetLocalTrust200JSONResponse(*inline), nil
}

func (svr *StrictServerImpl) getLocalTrust(
	_ context.Context, id openapi.LocalTrustId,
) (*openapi.InlineLocalTrust, error) {
	tm, ok := svr.core.StoredTrustMatrices.Load(id)
	if !ok {
		return nil, server.HTTPError{Code: 404}
	}
	var (
		result openapi.InlineLocalTrust
		err    error
	)
	err = tm.LockAndRun(func(c *sparse.Matrix, timestamp *big.Int) error {
		result.Size, err = c.Dim()
		if err != nil {
			return errors.Wrapf(err, "cannot get dimension")
		}
		result.Entries = make([]openapi.InlineLocalTrustEntry, 0, c.NNZ())
		for i, row := range c.Entries {
			for _, entry := range row {
				result.Entries = append(result.Entries,
					openapi.InlineLocalTrustEntry{
						I: i,
						J: entry.Index,
						V: entry.Value,
					})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (svr *StrictServerImpl) HeadLocalTrust(
	_ /*ctx*/ context.Context, request openapi.HeadLocalTrustRequestObject,
) (openapi.HeadLocalTrustResponseObject, error) {
	_, ok := svr.core.StoredTrustMatrices.Load(request.Id)
	if !ok {
		return openapi.HeadLocalTrust404Response{}, nil
	} else {
		return openapi.HeadLocalTrust204Response{}, nil
	}
}

func (svr *StrictServerImpl) UpdateLocalTrust(
	ctx context.Context, request openapi.UpdateLocalTrustRequestObject,
) (openapi.UpdateLocalTrustResponseObject, error) {
	logger := svr.core.Logger.With().
		Str("func", "(*StrictServerImpl).UpdateLocalTrust").
		Str("localTrustId", request.Id).
		Logger()
	c, err := svr.loadLocalTrust(request.Body)
	if err != nil {
		return nil, server.HTTPError{
			Code: 400, Inner: errors.Wrap(err, "cannot load local trust"),
		}
	}
	cDim, err := c.Dim()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get dimension")
	}
	logger.Trace().
		Int("dim", cDim).
		Int("nnz", c.NNZ()).
		Msg("local trust loaded")
	var (
		tm      *server.TrustMatrix
		created bool
	)
	if request.Params.Merge != nil && *request.Params.Merge {
		tm, created = svr.core.StoredTrustMatrices.Merge(request.Id, c)
	} else {
		tm, created = svr.core.StoredTrustMatrices.Set(request.Id, c)
	}
	_ = tm.LockAndRun(func(c *sparse.Matrix, timestamp *big.Int) error {
		err1 := c.Mmap(ctx)
		if err1 != nil {
			logger.Err(err1).Msg("cannot swap out local trust")
		}
		return nil
	})
	if created {
		return openapi.UpdateLocalTrust201Response{}, nil
	} else {
		return openapi.UpdateLocalTrust200Response{}, nil
	}
}

func (svr *StrictServerImpl) DeleteLocalTrust(
	_ /*ctx*/ context.Context, request openapi.DeleteLocalTrustRequestObject,
) (openapi.DeleteLocalTrustResponseObject, error) {
	_, deleted := svr.core.StoredTrustMatrices.LoadAndDelete(request.Id)
	if !deleted {
		return openapi.DeleteLocalTrust404Response{}, nil
	}
	return openapi.DeleteLocalTrust204Response{}, nil
}

func (svr *StrictServerImpl) loadLocalTrust(
	ref *openapi.LocalTrustRef,
) (*sparse.Matrix, error) {
	switch ref.Scheme {
	case "inline":
		inline, err := ref.AsInlineLocalTrust()
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot parse inline local trust reference")
		}
		return svr.loadInlineLocalTrust(&inline)
	case "stored":
		stored, err := ref.AsStoredLocalTrust()
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot parse stored local trust reference")
		}
		return svr.loadStoredLocalTrust(&stored)
	default:
		return nil, errors.Errorf("unknown local trust ref type %#v",
			ref.Scheme)
	}
}

func (svr *StrictServerImpl) loadInlineLocalTrust(
	inline *openapi.InlineLocalTrust,
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
	return sparse.NewCSRMatrix(size, size, entries, false), nil
}

func (svr *StrictServerImpl) loadStoredLocalTrust(
	stored *openapi.StoredLocalTrust,
) (c *sparse.Matrix, err error) {
	tm0, ok := svr.core.StoredTrustMatrices.Load(stored.Id)
	if ok {
		// Caller may modify returned c in-place (canonicalize, size-match)
		// so return a disposable copy, preserving the original.
		// This is slow: It takes ~3s to copy 16M nonzero entries.
		// TODO(ek): Implement on-demand canonicalization and remove this.
		_ = tm0.LockAndRun(func(c0 *sparse.Matrix, timestamp *big.Int) error {
			c = deepcopy.Copy(c0).(*sparse.Matrix)
			return nil
		})
	} else {
		err = server.HTTPError{
			Code: 400, Inner: errors.New("local trust not found"),
		}
	}
	return
}

func loadTrustVector(ref *openapi.TrustVectorRef) (*sparse.Vector, error) {
	if inline, err := ref.AsInlineTrustVector(); err == nil {
		return loadInlineTrustVector(&inline)
	}
	return nil, errors.New("unknown trust vector type")
}

func loadInlineTrustVector(inline *openapi.InlineTrustVector) (
	*sparse.Vector, error,
) {
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
