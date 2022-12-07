package basic

import (
	"context"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type StrictServerImpl struct {
	Logger zerolog.Logger
}

func (server StrictServerImpl) Compute(
	ctx context.Context, request ComputeRequestObject,
) (ComputeResponseObject, error) {
	var (
		localTrust LocalTrust
		preTrust   TrustVector
		alpha      float64
		epsilon    float64
		err        error
	)
	if localTrust, err = loadLocalTrust(&request.Body.LocalTrust); err != nil {
		return nil, errors.Wrap(err, "cannot load local trust")
	}
	if request.Body.PreTrust == nil {
		// Default to zero pre-trust (canonicalized into uniform later).
		preTrust = NewEmptyTrustVector().Grow(localTrust.Dim())
	} else if preTrust, err = loadTrustVector(request.Body.PreTrust); err != nil {
		return nil, errors.Wrap(err, "cannot load pre-trust")
	}
	if localTrust.Dim() != preTrust.Len() {
		return nil, errors.Errorf("local trust size %d != pre-trust size %d",
			localTrust.Dim(), preTrust.Len())
	}
	if request.Body.Alpha == nil {
		alpha = 0.5
	} else {
		alpha = *request.Body.Alpha
		if alpha < 0 || alpha > 1 {
			return nil, errors.Errorf("alpha=%f out of range [0..1]", alpha)
		}
	}
	if request.Body.Epsilon == nil {
		epsilon = 1e-6 / float64(localTrust.Dim())
	} else {
		epsilon = *request.Body.Epsilon
		if epsilon <= 0 || epsilon > 1 {
			return nil, errors.Errorf("epsilon=%f out of range (0..1]", epsilon)
		}
	}
	p := preTrust.Canonicalize()
	c, err := localTrust.Canonicalize(p)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot canonicalize local trust")
	}
	t, err := Compute(ctx, c, p, alpha, epsilon, nil, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot compute EigenTrust")
	}
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

func loadLocalTrust(ref *LocalTrustRef) (LocalTrust, error) {
	if inline, err := ref.AsInlineLocalTrust(); err == nil {
		return loadInlineLocalTrust(&inline)
	}
	return nil, errors.New("unknown local trust ref type")
}

func loadInlineLocalTrust(inline *InlineLocalTrust) (LocalTrust, error) {
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
