package server

import (
	"math/big"
	"sync"

	"k3l.io/go-eigentrust/pkg/sparse"
)

type TrustVector struct {
	vector    *sparse.Vector
	timestamp big.Int
	mutex     sync.Mutex
}

func NewTrustVectorWithContents(c *sparse.Vector) *TrustVector {
	return &TrustVector{vector: c}
}

func NewTrustVector() *TrustVector {
	return NewTrustVectorWithContents(sparse.NewVector(0, nil))
}

func (m *TrustVector) LockAndRun(
	f func(vector *sparse.Vector, timestamp *big.Int) error,
) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return f(m.vector, &m.timestamp)
}

type TrustMatrix struct {
	matrix    *sparse.Matrix
	timestamp big.Int
	mutex     sync.Mutex
}

func NewTrustMatrixWithContents(c *sparse.Matrix) *TrustMatrix {
	return &TrustMatrix{matrix: c}
}

func NewTrustMatrix() *TrustMatrix {
	return NewTrustMatrixWithContents(sparse.NewCSRMatrix(0, 0, nil, false))
}

func (m *TrustMatrix) LockAndRun(
	f func(matrix *sparse.Matrix, timestamp *big.Int) error,
) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return f(m.matrix, &m.timestamp)
}
