package basic

import "C"
import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"k3l.io/go-eigentrust/pkg/sparse"
)

type StrictServerImpl struct {
	logger              zerolog.Logger
	storedLocalTrust    map[LocalTrustId]*sparse.Matrix
	storedLocalTrustMtx sync.Mutex
}

func NewStrictServerImpl(logger zerolog.Logger) *StrictServerImpl {
	return &StrictServerImpl{
		logger:           logger,
		storedLocalTrust: make(map[LocalTrustId]*sparse.Matrix),
	}
}

type HTTPError struct {
	Code  int
	Inner error
}

func (e HTTPError) Error() string {
	statusText := http.StatusText(e.Code)
	if statusText != "" {
		statusText = " " + statusText
	}
	return fmt.Sprintf("HTTP %d%s: %s", e.Code, statusText, e.Inner.Error())
}

func (server *StrictServerImpl) compute(
	ctx context.Context, localTrustRef *LocalTrustRef,
	initialTrust *TrustVectorRef, preTrust *TrustVectorRef,
	alpha *float64, epsilon *float64,
	flatTail *int, numLeaders *int,
	maxIterations *int,
) (tv TrustVectorRef, flatTailStats FlatTailStats, err error) {
	logger := server.logger.With().
		Str("func", "(*StrictServerImpl).Compute").
		Logger()
	var (
		c  *sparse.Matrix
		p  *sparse.Vector
		t0 *sparse.Vector
	)
	opts := []ComputeOpt{WithFlatTailStats(&flatTailStats)}
	if c, err = server.loadLocalTrust(localTrustRef); err != nil {
		err = HTTPError{400, errors.Wrap(err, "cannot load local trust")}
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
		err = HTTPError{400, errors.Wrap(err, "cannot load pre-trust")}
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
		err = HTTPError{400, errors.Wrap(err, "cannot load initial trust")}
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
		opts = append(opts, WithInitialTrust(t0))
	}
	if alpha == nil {
		a := 0.5
		alpha = &a
	} else if *alpha < 0 || *alpha > 1 {
		err = HTTPError{
			400, errors.Errorf("alpha=%f out of range [0..1]", *alpha),
		}
		return
	}
	if epsilon == nil {
		e := 1e-6 / float64(cDim)
		epsilon = &e
	} else if *epsilon <= 0 || *epsilon > 1 {
		err = HTTPError{
			400, errors.Errorf("epsilon=%f out of range (0..1]", *epsilon),
		}
		return
	}
	if flatTail != nil {
		opts = append(opts, WithFlatTail(*flatTail))
	}
	if numLeaders != nil {
		opts = append(opts, WithFlatTailNumLeaders(*numLeaders))
	}
	if maxIterations != nil {
		opts = append(opts, WithMaxIterations(*maxIterations))
	}
	CanonicalizeTrustVector(p)
	if t0 != nil {
		CanonicalizeTrustVector(t0)
	}
	err = CanonicalizeLocalTrust(c, p)
	if err != nil {
		err = HTTPError{
			400, errors.Wrapf(err, "cannot canonicalize local trust"),
		}
		return
	}
	t, err := Compute(ctx, c, p, *alpha, *epsilon, opts...)
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
	ctx = server.logger.WithContext(ctx)
	tv, _, err := server.compute(ctx,
		&request.Body.LocalTrust,
		request.Body.InitialTrust, request.Body.PreTrust,
		request.Body.Alpha, request.Body.Epsilon,
		request.Body.FlatTail, request.Body.NumLeaders,
		request.Body.MaxIterations)
	if err != nil {
		if httpError, ok := err.(HTTPError); ok {
			switch httpError.Code {
			case 400:
				var resp Compute400JSONResponse
				resp.Message = httpError.Inner.Error()
				return resp, nil
			}
		}
		return nil, err
	}
	return Compute200JSONResponse(tv), nil
}

func (server *StrictServerImpl) ComputeWithStats(
	ctx context.Context, request ComputeWithStatsRequestObject,
) (ComputeWithStatsResponseObject, error) {
	ctx = server.logger.WithContext(ctx)
	tv, flatTailStats, err := server.compute(ctx,
		&request.Body.LocalTrust,
		request.Body.InitialTrust, request.Body.PreTrust,
		request.Body.Alpha, request.Body.Epsilon,
		request.Body.FlatTail, request.Body.NumLeaders,
		request.Body.MaxIterations)
	if err != nil {
		if httpError, ok := err.(HTTPError); ok {
			switch httpError.Code {
			case 400:
				var resp ComputeWithStats400JSONResponse
				resp.Message = httpError.Inner.Error()
				return resp, nil
			}
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

func (server *StrictServerImpl) GetLocalTrust(
	ctx context.Context, request GetLocalTrustRequestObject,
) (GetLocalTrustResponseObject, error) {
	inline, err := server.getLocalTrust(ctx, request.Id)
	if err != nil {
		if httpError, ok := err.(HTTPError); ok {
			switch httpError.Code {
			case 404:
				return GetLocalTrust404Response{}, nil
			}
		}
		return nil, err
	}
	return GetLocalTrust200JSONResponse(*inline), nil
}

func (server *StrictServerImpl) getLocalTrust(
	_ context.Context, id LocalTrustId,
) (*InlineLocalTrust, error) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	c := server.storedLocalTrust[id]
	if c == nil {
		return nil, HTTPError{Code: 404}
	}
	cDim, err := c.Dim()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get dimension")
	}
	entries := make([]InlineLocalTrustEntry, 0, c.NNZ())
	for i, row := range c.Entries {
		for _, entry := range row {
			entries = append(entries, InlineLocalTrustEntry{
				I: i,
				J: entry.Index,
				V: entry.Value,
			})
		}
	}
	return &InlineLocalTrust{
		Entries: entries,
		Size:    cDim,
	}, nil
}

func (server *StrictServerImpl) HeadLocalTrust(
	ctx context.Context, request HeadLocalTrustRequestObject,
) (HeadLocalTrustResponseObject, error) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	if server.storedLocalTrust[request.Id] == nil {
		return HeadLocalTrust404Response{}, nil
	} else {
		return HeadLocalTrust204Response{}, nil
	}
}

func (server *StrictServerImpl) UpdateLocalTrust(
	ctx context.Context, request UpdateLocalTrustRequestObject,
) (UpdateLocalTrustResponseObject, error) {
	logger := server.logger.With().
		Str("func", "(*StrictServerImpl).UpdateLocalTrust").
		Str("localTrustId", request.Id).
		Logger()
	c, err := server.loadLocalTrust(request.Body)
	if err != nil {
		return nil, HTTPError{400, errors.Wrap(err, "cannot load local trust")}
	}
	cDim, err := c.Dim()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get dimension")
	}
	logger.Trace().
		Int("dim", cDim).
		Int("nnz", c.NNZ()).
		Msg("local trust loaded")
	err = c.Mmap(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot swap out local trust")
	}
	if server.setStoredLocalTrust(request.Id, c) {
		return UpdateLocalTrust201Response{}, nil
	} else {
		return UpdateLocalTrust200Response{}, nil
	}
}

func (server *StrictServerImpl) DeleteLocalTrust(
	ctx context.Context, request DeleteLocalTrustRequestObject,
) (DeleteLocalTrustResponseObject, error) {
	if !server.deleteStoredLocalTrust(request.Id) {
		return DeleteLocalTrust404Response{}, nil
	}
	return DeleteLocalTrust204Response{}, nil
}

func (server *StrictServerImpl) loadLocalTrust(
	ref *LocalTrustRef,
) (*sparse.Matrix, error) {
	switch ref.Scheme {
	case "inline":
		inline, err := ref.AsInlineLocalTrust()
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot parse inline local trust reference")
		}
		return server.loadInlineLocalTrust(&inline)
	case "stored":
		stored, err := ref.AsStoredLocalTrust()
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot parse stored local trust reference")
		}
		return server.loadStoredLocalTrust(&stored)
	default:
		return nil, errors.New("unknown local trust ref type")
	}
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

func (server *StrictServerImpl) loadStoredLocalTrust(
	stored *StoredLocalTrust,
) (*sparse.Matrix, error) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	c, ok := server.storedLocalTrust[stored.Id]
	if !ok {
		return nil, HTTPError{400, errors.New("local trust not found")}
	}
	// Returned c is modified in-place (canonicalized, size-matched)
	// so return a disposable copy, preserving the original.
	// This is slow: It takes ~3s to copy 16M nonzero entries.
	// TODO(ek): Implement on-demand canonicalization and remove this.
	c = deepcopy.Copy(c).(*sparse.Matrix)
	return c, nil
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

func (server *StrictServerImpl) setStoredLocalTrust(
	id LocalTrustId, c *sparse.Matrix,
) (created bool) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	_, ok := server.storedLocalTrust[id]
	server.storedLocalTrust[id] = c
	return !ok
}
func (server *StrictServerImpl) deleteStoredLocalTrust(
	id LocalTrustId,
) (deleted bool) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	if _, ok := server.storedLocalTrust[id]; ok {
		delete(server.storedLocalTrust, id)
		deleted = true
	}
	return
}
