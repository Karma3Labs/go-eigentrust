package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"time"

	"github.com/rs/zerolog"
	"k3l.io/go-eigentrust/pkg/sparse"
	"k3l.io/go-eigentrust/pkg/util"
)

type NamedTrustMatrices struct {
	util.SyncMap[string, *TrustMatrix]
}

// New creates and stores an empty matrix under a random name.
func (ntms *NamedTrustMatrices) New(ctx context.Context) (
	id string, err error,
) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	tm := NewTrustMatrix()
	for {
		id, err = RandomId(ctx)
		if err != nil {
			return "", err
		}
		_, loaded := ntms.SyncMap.LoadOrStore(id, tm)
		if !loaded {
			return id, nil
		}
	}
}

// NewNamed creates and stores an empty vector under the given name.
func (ntms *NamedTrustMatrices) NewNamed(id string) error {
	if _, loaded := ntms.SyncMap.LoadOrStore(id, NewTrustMatrix()); loaded {
		return fmt.Errorf("already have a trust matrix %q", id)
	}
	return nil
}

// Set stores c into the stored local trust.
// It takes ownership of c; caller must not use c anymore.
func (ntms *NamedTrustMatrices) Set(
	id string, c *sparse.Matrix,
) (tm *TrustMatrix, created bool) {
	tm = NewTrustMatrixWithContents(c)
	_, loaded := ntms.Swap(id, tm)
	created = !loaded
	return
}

// Merge merges c into the stored local trust.
// It takes ownership of c; caller must not use c anymore.
func (ntms *NamedTrustMatrices) Merge(
	id string, c *sparse.Matrix,
) (tm2 *TrustMatrix, created bool) {
	tm1 := NewTrustMatrixWithContents(c)
	tm2, loaded := ntms.LoadOrStore(id, tm1)
	if tm2 != tm1 {
		_ = tm2.LockAndRun(func(c2 *sparse.Matrix, timestamp *big.Int) error {
			c2.Merge(&c.CSMatrix)
			return nil
		})
		c.Reset()
	}
	return tm2, !loaded
}

func (ntms *NamedTrustMatrices) Delete(id string) (deleted bool) {
	_, deleted = ntms.LoadAndDelete(id)
	return
}

type NamedTrustVectors struct {
	util.SyncMap[string, *TrustVector]
}

// New creates and stores an empty vector under a random name.
func (ntvs *NamedTrustVectors) New(ctx context.Context) (id string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	tm := NewTrustVector()
	for {
		id, err = RandomId(ctx)
		if err != nil {
			return "", err
		}
		_, loaded := ntvs.SyncMap.LoadOrStore(id, tm)
		if !loaded {
			return id, nil
		}
	}
}

// NewNamed creates and stores an empty vector under the given name.
func (ntvs *NamedTrustVectors) NewNamed(id string) error {
	if _, loaded := ntvs.SyncMap.LoadOrStore(id, NewTrustVector()); loaded {
		return fmt.Errorf("already have a trust vector %q", id)
	}
	return nil
}

// Set stores v into the stored local trust.
// It takes ownership of v; caller must not use v anymore.
func (ntvs *NamedTrustVectors) Set(
	id string, v *sparse.Vector,
) (tv *TrustVector, created bool) {
	tv = NewTrustVectorWithContents(v)
	_, loaded := ntvs.Swap(id, tv)
	created = !loaded
	return
}

// Merge merges v into the stored local trust.
// It takes ownership of v; caller must not use v anymore.
func (ntvs *NamedTrustVectors) Merge(
	id string, v *sparse.Vector,
) (tv2 *TrustVector, created bool) {
	tv1 := NewTrustVectorWithContents(v)
	tv2, loaded := ntvs.LoadOrStore(id, tv1)
	if tv2 != tv1 {
		_ = tv2.LockAndRun(func(v2 *sparse.Vector, timestamp *big.Int) error {
			v2.Merge(v)
			return nil
		})
		v.Reset()
	}
	return tv2, !loaded
}

func (ntvs *NamedTrustVectors) Delete(id string) (deleted bool) {
	_, deleted = ntvs.LoadAndDelete(id)
	return
}

func RandomId(ctx context.Context) (id string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	buf := make([]byte, 24)
	delay := 125 * time.Millisecond
	for _, err = rand.Read(buf); err != nil; _, err = rand.Read(buf) {
		zerolog.Ctx(ctx).Err(err).Msg("cannot create random name")
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		case <-time.After(delay):
			delay *= 2
		}
	}
	id = base64.StdEncoding.EncodeToString(buf)
	return
}
