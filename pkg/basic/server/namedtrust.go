package server

import (
	"math/big"

	"k3l.io/go-eigentrust/pkg/sparse"
	"k3l.io/go-eigentrust/pkg/util"
)

type NamedTrustMatrices struct {
	util.SyncMap[string, *TrustMatrix]
}

// Set stores c into the stored local trust.
// It takes ownership of c; caller must not use c anymore.
func (ntm *NamedTrustMatrices) Set(
	id string, c *sparse.Matrix,
) (tm *TrustMatrix, created bool) {
	tm = NewTrustMatrixWithContents(c)
	_, loaded := ntm.Swap(id, tm)
	created = !loaded
	return
}

// Merge merges c into the stored local trust.
// It takes ownership of c; caller must not use c anymore.
func (ntm *NamedTrustMatrices) Merge(
	id string, c *sparse.Matrix,
) (tm2 *TrustMatrix, created bool) {
	tm1 := NewTrustMatrixWithContents(c)
	tm2, loaded := ntm.LoadOrStore(id, tm1)
	if tm2 != tm1 {
		tm2.LockAndRun(func(c2 *sparse.Matrix, timestamp *big.Int) {
			c2.Merge(&c.CSMatrix)
		})
		c.Reset()
	}
	return tm2, !loaded
}

func (ntm *NamedTrustMatrices) Delete(id string) (deleted bool) {
	_, deleted = ntm.LoadAndDelete(id)
	return
}
