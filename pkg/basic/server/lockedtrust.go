package server

import (
	"math/big"
	"sync"

	"k3l.io/go-eigentrust/pkg/sparse"
)

type TrustMatrix struct {
	matrix    *sparse.Matrix
	timestamp big.Int
	mutex     sync.Mutex
}

func NewTrustMatrixWithContents(c *sparse.Matrix) *TrustMatrix {
	return &TrustMatrix{matrix: c}
}

func NewTrustMatrix() *TrustMatrix {
	return NewTrustMatrixWithContents(sparse.NewCSRMatrix(0, 0, nil))
}

func (m *TrustMatrix) LockAndRun(
	f func(matrix *sparse.Matrix, timestamp *big.Int),
) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	f(m.matrix, &m.timestamp)
}
