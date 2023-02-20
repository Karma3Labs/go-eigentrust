package basic

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
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
	logger := server.Logger.With().
		Str("name", "(*StrictServerImpl).Compute").Logger()
	logTime := NewWallTimeLogger(logger).Log
	defer logTime("last")
	var (
		localTrust LocalTrust
		preTrust   TrustVector
		alpha      float64
		epsilon    float64
		err        error
	)
	if localTrust, err = server.loadLocalTrust(&request.Body.LocalTrust); err != nil {
		return wrapIn400(err, "cannot load local trust"), nil
	}
	logTime("loadLocalTrust")
	if request.Body.PreTrust == nil {
		// Default to zero pre-trust (canonicalized into uniform later).
		preTrust = NewEmptyTrustVector().Grow(localTrust.Dim())
	} else if preTrust, err = loadTrustVector(request.Body.PreTrust); err != nil {
		return wrapIn400(err, "cannot load pre-trust"), nil
	}
	if localTrust.Dim() != preTrust.Len() {
		return format400("local trust size %d != pre-trust size %d",
			localTrust.Dim(), preTrust.Len()), nil
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
		epsilon = 1e-6 / float64(localTrust.Dim())
	} else {
		epsilon = *request.Body.Epsilon
		if epsilon <= 0 || epsilon > 1 {
			return format400("epsilon=%f out of range (0..1]", epsilon), nil
		}
	}
	logTime("preprocessing")
	p := preTrust.Canonicalize()
	logTime("CanonicalizePreTrust")
	c, err := localTrust.Canonicalize(p)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot canonicalize local trust")
	}
	logTime("CanonicalizeLocalTrust")
	t, err := Compute(ctx, c, p, alpha, epsilon, nil, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot compute EigenTrust")
	}
	logTime("Compute")
	var itv InlineTrustVector
	itv.Scheme = "inline" // FIXME(ek): can we not hard-code this?
	itv.Size = t.Len()
	for i, v := range t.Components() {
		if v != 0 {
			itv.Entries = append(itv.Entries,
				InlineTrustVectorEntry{I: i, V: v})
		}
	}
	var tv TrustVectorRef
	if err = tv.FromInlineTrustVector(itv); err != nil {
		return nil, errors.Wrapf(err, "cannot create response")
	}
	return Compute200JSONResponse(tv), nil
}

func (server *StrictServerImpl) loadLocalTrust(
	ref *LocalTrustRef,
) (LocalTrust, error) {
	if inline, err := ref.AsInlineLocalTrust(); err == nil {
		return server.loadInlineLocalTrust(&inline)
	}
	return nil, errors.New("unknown local trust ref type")
}

func (server *StrictServerImpl) loadInlineLocalTrust(
	inline *InlineLocalTrust,
) (LocalTrust, error) {
	if inline.Size <= 0 {
		return nil, errors.Errorf("invalid size=%#v", inline.Size)
	}
	lt := NewEmptyLocalTrust().Grow(inline.Size)
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
		lt.Set(entry.I, entry.J, entry.V)
	}
	return lt, nil
}

func loadTrustVector(ref *TrustVectorRef) (TrustVector, error) {
	if inline, err := ref.AsInlineTrustVector(); err == nil {
		return loadInlineTrustVector(&inline)
	}
	return nil, errors.New("unknown trust vector type")
}

func loadInlineTrustVector(inline *InlineTrustVector) (TrustVector, error) {
	lt := NewEmptyTrustVector().Grow(inline.Size)
	for idx, entry := range inline.Entries {
		if entry.I < 0 || entry.I >= inline.Size {
			return nil, errors.Errorf("entry %d: i=%d is out of range [0..%d)",
				idx, entry.I, inline.Size)
		}
		if entry.V <= 0 {
			return nil, errors.Errorf("entry %d: v=%f is out of range (0, inf)",
				idx, entry.V)
		}
		lt.SetVec(entry.I, entry.V)
	}
	return lt, nil
}
