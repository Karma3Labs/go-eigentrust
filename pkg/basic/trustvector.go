package basic

import (
	"io"

	"github.com/pkg/errors"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

// TrustVector is a trust vector.
type TrustVector interface {
	mat.MutableVector

	// Components returns the raw components slice.
	Components() []float64

	// Cap returns the receiver's capacity.
	Cap() int

	// Grow returns the receiver expanded by n rows and n columns.
	// See [mat.Dense.Grow] for details.
	Grow(n int) TrustVector

	// Canonicalize canonicalizes the receiver,
	// i.e. scales it so that the components sum to one,
	// or if the receiver is a zero vector,
	// a uniform vector that sums to one is used instead.
	//
	// Canonicalize returns the canonicalized vector;
	// the receiver remains unchanged.
	Canonicalize() TrustVector
}

type trustVector struct {
	c []float64
	v *mat.VecDense
}

func (p *trustVector) Dims() (r, c int) {
	return p.v.Dims()
}

func (p *trustVector) At(i, j int) float64 {
	return p.v.At(i, j)
}

func (p *trustVector) T() mat.Matrix {
	return p.v.T()
}

func (p *trustVector) AtVec(i int) float64 {
	return p.v.AtVec(i)
}

func (p *trustVector) Len() int {
	return p.v.Len()
}

func (p *trustVector) SetVec(i int, v float64) {
	p.v.SetVec(i, v)
}

func (p *trustVector) Components() []float64 {
	return p.c
}

func (p *trustVector) Cap() int {
	return cap(p.c)
}

func (p *trustVector) Grow(n int) TrustVector {
	if n < 0 {
		panic("negative trust vector grow amount")
	}
	if n == 0 {
		return p
	}
	c := p.c
	n += len(c)
	if cap(c) < n {
		c0 := c
		c = make([]float64, n*5/4) // 125% target size
		copy(c, c0)
	}
	c = c[:n]
	return &trustVector{
		c: c,
		v: mat.NewVecDense(n, c),
	}
}

func (p *trustVector) Canonicalize() TrustVector {
	n := len(p.c)
	c := make([]float64, n)
	switch _, err := Canonicalize(p.c, c); err {
	case nil:
	case ErrZeroSum:
		floats.AddConst(1/float64(n), c)
	}
	return &trustVector{c: c, v: mat.NewVecDense(n, c)}
}

func NewEmptyTrustVector() TrustVector {
	return &trustVector{
		c: nil, v: &mat.VecDense{},
	}
}

func NewTrustVector(n int, data []float64) TrustVector {
	if data == nil {
		data = make([]float64, n)
	}
	return &trustVector{c: data, v: mat.NewVecDense(n, data)}
}

func TrustVectorCopyOf(t TrustVector) TrustVector {
	if t == nil {
		return nil
	}
	c := append([]float64(nil), t.Components()...)
	return &trustVector{c: c, v: mat.NewVecDense(len(c), c)}
}

// ReadTrustVectorFromCsv reads a trust vector from the given CSV file.
func ReadTrustVectorFromCsv(
	reader CsvReader, peerIndices map[string]int,
) (TrustVector, error) {
	parseFields := func(fields []string) (peer int, level float64, err error) {
		peer = -1
		if len(fields) < 1 {
			err = errors.New("too few fields")
		} else if peer, err = ParsePeerId(fields[0], peerIndices); err != nil {
			err = errors.Wrapf(err, "invalid peer %#v", fields[0])
		} else if len(fields) >= 2 {
			if level, err = ParseTrustLevel(fields[1]); err != nil {
				err = errors.Wrapf(err, "invalid trust level %#v", fields[1])
			}
		} else {
			level = 1.0
		}
		return
	}
	pt := NewEmptyTrustVector()
	count := 0
	fields, err := reader.Read()
	for ; err == nil; fields, err = reader.Read() {
		count++
		peer, level, err := parseFields(fields)
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot parse trust vector CSV record #%d", count)
		}
		n := peer + 1
		if pt.Len() < n {
			pt = pt.Grow(n - pt.Len())
		}
		pt.SetVec(peer, level)
	}
	if err != io.EOF {
		return nil, errors.Wrapf(err,
			"cannot read trust vector CSV record #%d", count+1)
	}
	return pt, nil
}
