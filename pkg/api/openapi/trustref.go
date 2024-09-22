package openapi

import (
	"context"

	"k3l.io/go-eigentrust/pkg/sparse"
	spopt "k3l.io/go-eigentrust/pkg/sparse/option"
	"k3l.io/go-eigentrust/pkg/util"
)

func CooFromInlineEntry(
	ite InlineTrustEntry,
) (*sparse.CooEntry, error) {
	ij, err := ite.AsTrustMatrixEntryIndices()
	if err != nil {
		return nil, err
	}
	return &sparse.CooEntry{Row: ij.I, Column: ij.J, Value: ite.V}, nil
}

// MatrixFromInline converts the given inline trust ref into a matrix.
func MatrixFromInline(
	ctx context.Context, ref *InlineTrustRef, opts ...spopt.Option,
) (*sparse.Matrix, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan sparse.CooEntry)
	send := func() error {
		for _, ite := range ref.Entries {
			coo, err := CooFromInlineEntry(ite)
			if err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- *coo:
			}
		}
		return nil
	}
	sendErr := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(sendErr)
		sendErr <- send()
	}()
	opts = append(opts, spopt.FixedDim(ref.Size, ref.Size))
	m, err := sparse.NewCSRMatrixFromEntryCh(ctx, ch, opts...)
	if err == nil {
		err = util.ErrFromCh(ctx, sendErr)
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// InlineFromMatrix converts the given square matrix into an inline trust ref.
func InlineFromMatrix(
	ctx context.Context, m *sparse.Matrix, opts ...spopt.Option,
) (*InlineTrustRef, error) {
	ch := make(chan sparse.CooEntry)
	sendErr := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(sendErr)
		sendErr <- m.SendCooEntries(ctx, ch, opts...)
	}()
	coos, err := util.ReceiveElements(ctx, ch)
	if err == nil {
		err = util.ErrFromCh(ctx, sendErr)
	}
	if err != nil {
		return nil, err
	}
	ites, err := util.MapWithErr(coos,
		func(entry sparse.CooEntry) (ite InlineTrustEntry, err error) {
			ite.V = entry.Value
			err = ite.FromTrustMatrixEntryIndices(TrustMatrixEntryIndices{
				I: entry.Row, J: entry.Column,
			})
			return
		})
	if err != nil {
		return nil, err
	}
	size, err := m.Dim()
	if err != nil {
		return nil, err
	}
	return &InlineTrustRef{Entries: ites, Size: size}, nil
}

func SparseFromInlineEntry(
	ite InlineTrustEntry,
) (*sparse.Entry, error) {
	i, err := ite.AsTrustVectorEntryIndex()
	if err != nil {
		return nil, err
	}
	return &sparse.Entry{Index: i.I, Value: ite.V}, nil
}

// VectorFromInline converts the given inline trust ref into a vector.
func VectorFromInline(
	ctx context.Context, ref *InlineTrustRef, opts ...spopt.Option,
) (*sparse.Vector, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan sparse.Entry)
	send := func() error {
		for _, ite := range ref.Entries {
			entry, err := SparseFromInlineEntry(ite)
			if err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- *entry:
			}
		}
		return nil
	}
	sendErr := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(sendErr)
		sendErr <- send()
	}()
	v, err := sparse.NewVectorFromEntryCh(ctx, ch, opts...)
	if err == nil {
		err = util.ErrFromCh(ctx, sendErr)
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

// InlineFromVector converts the given vector into an inline trust ref.
func InlineFromVector(
	ctx context.Context, v *sparse.Vector, opts ...spopt.Option,
) (*InlineTrustRef, error) {
	ch := make(chan sparse.Entry)
	sendErr := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(sendErr)
		sendErr <- v.SendEntries(ctx, ch, opts...)
	}()
	entries, err := util.ReceiveElements(ctx, ch)
	if err == nil {
		err = util.ErrFromCh(ctx, sendErr)
	}
	if err != nil {
		return nil, err
	}
	ites, err := util.MapWithErr(entries,
		func(entry sparse.Entry) (ite InlineTrustEntry, err error) {
			ite.V = entry.Value
			err = ite.FromTrustVectorEntryIndex(TrustVectorEntryIndex{I: entry.Index})
			return
		})
	if err != nil {
		return nil, err
	}
	return &InlineTrustRef{Entries: ites, Size: v.Dim}, nil
}
