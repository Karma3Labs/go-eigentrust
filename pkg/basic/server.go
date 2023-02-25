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

func make400(message string) (resp Compute400JSONResponse) {
	resp.Message = message
	return
}

func format400(format string, v ...interface{}) (resp Compute400JSONResponse) {
	resp.Message = fmt.Sprintf(format, v...)
	return
}

func errorTo400(err error) (resp Compute400JSONResponse) {
	return make400(err.Error())
}

func wrapIn400(err error, message string) (resp Compute400JSONResponse) {
	return errorTo400(errors.Wrap(err, message))
}

func wrapfIn400(
	err error, format string, v ...interface{},
) (resp Compute400JSONResponse) {
	return errorTo400(errors.Wrapf(err, format, v...))
}

func (server *StrictServerImpl) Compute(
	ctx context.Context, request ComputeRequestObject,
) (ComputeResponseObject, error) {
	ctx = util.SetLoggerInContext(ctx, server.Logger)
	logger := server.Logger.With().
		Str("func", "(*StrictServerImpl).Compute").
		Logger()
	var (
		c       *sparse.Matrix
		p       *sparse.Vector
		alpha   float64
		epsilon float64
		err     error
	)
	if c, err = server.loadLocalTrust(&request.Body.LocalTrust); err != nil {
		return wrapIn400(err, "cannot load local trust"), nil
	}
	cDim, err := c.Dim()
	if err != nil {
		return nil, err
	}
	logger.Trace().
		Int("dim", cDim).
		Int("nnz", c.NNZ()).
		Msg("local trust loaded")
	if request.Body.PreTrust == nil {
		// Default to zero pre-trust (canonicalized into uniform later).
		p = sparse.NewVector(cDim, nil)
	} else if p, err = loadTrustVector(request.Body.PreTrust); err != nil {
		return wrapIn400(err, "cannot load pre-trust"), nil
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
	if cDim != p.Dim {
		return format400("local trust size %d != pre-trust size %d",
			cDim, p.Dim), nil
	}
	if request.Body.Alpha == nil {
		alpha = 0.5
	} else {
		alpha = *request.Body.Alpha
		if alpha < 0 || alpha > 1 {
			return format400("alpha=%f out of range [0..1]", alpha), nil
		}
	}
	if request.Body.Epsilon == nil {
		epsilon = 1e-6 / float64(cDim)
	} else {
		epsilon = *request.Body.Epsilon
		if epsilon <= 0 || epsilon > 1 {
			return format400("epsilon=%f out of range (0..1]", epsilon), nil
		}
	}
	CanonicalizeTrustVector(p)
	err = CanonicalizeLocalTrust(c, p)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot canonicalize local trust")
	}
	t, err := Compute(ctx, c, p, alpha, epsilon, nil, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot compute EigenTrust")
	}
	var itv InlineTrustVector
	itv.Scheme = "inline" // FIXME(ek): can we not hard-code this?
	itv.Size = t.Dim
	for _, e := range t.Entries {
		itv.Entries = append(itv.Entries,
			InlineTrustVectorEntry{I: e.Index, V: e.Value})
	}
	var tv TrustVectorRef
	if err = tv.FromInlineTrustVector(itv); err != nil {
		return nil, errors.Wrapf(err, "cannot create response")
	}
	c = nil
	p = nil
	runtime.GC()
	return Compute200JSONResponse(tv), nil
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
	return sparse.NewCSRMatrix(inline.Size, inline.Size, entries), nil
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
	return sparse.NewVector(inline.Size, entries), nil
}
