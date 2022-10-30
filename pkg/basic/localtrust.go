package basic

import (
	"io"

	"github.com/pkg/errors"
	"gonum.org/v1/gonum/mat"
)

// LocalTrust is the local trust matrix.
type LocalTrust interface {
	mat.Mutable

	// Dim returns the receiver's (square) dimension.
	Dim() int

	// Cap returns the receiver's (square) capacity.
	Cap() int

	// Grow returns the receiver expanded by n rows and n columns.
	// See [mat.Dense.Grow] for details.
	Grow(n int) LocalTrust

	// Canonicalize canonicalizes the receiver,
	// i.e. scales each row so that its entries sum to one.
	//
	// Zero rows are replaced by trustVector vector.
	//
	// Canonicalize returns the canonicalized matrix;
	// the receiver remains unchanged.
	//
	// The receiver and trustVector must have the same dimension.
	Canonicalize(preTrust TrustVector) (LocalTrust, error)
}

type localTrust struct {
	m *mat.Dense
}

func (l *localTrust) Set(i, j int, v float64) {
	if v < 0 {
		panic("negative local trust value")
	}
	l.m.Set(i, j, v)
}

func (l *localTrust) Dims() (r, c int) {
	return l.m.Dims()
}

func (l *localTrust) At(i, j int) float64 {
	return l.m.At(i, j)
}

func (l *localTrust) T() mat.Matrix {
	return l.m.T()
}

func (l *localTrust) Dim() int {
	n, _ := l.m.Dims()
	return n
}

func (l *localTrust) Cap() int {
	n, _ := l.m.Caps()
	return n
}

func (l *localTrust) Grow(n int) LocalTrust {
	// TODO(ek): Don't depend on that [mat.(*Dense).Grow] returns *mat.Dense.
	m := l.m.Grow(n, n).(*mat.Dense)
	if m == l.m {
		return l
	}
	return &localTrust{m: m}
}

func (l *localTrust) Canonicalize(preTrust TrustVector) (LocalTrust, error) {
	n := l.Dim()
	if n != preTrust.Len() {
		return nil, ErrDimensionMismatch
	}
	l2 := &localTrust{m: mat.NewDense(n, n, nil)}
	for i := 0; i < n; i++ {
		inRow := l.m.RawRowView(i)
		outRow := l2.m.RawRowView(i)
		switch _, err := Canonicalize(inRow, outRow); err {
		case nil:
		case ErrZeroSum:
			l2.m.SetRow(i, preTrust.Components())
		default:
			return nil, err
		}
	}
	return l2, nil
}

// NewEmptyLocalTrust creates and returns a new local trust object.
func NewEmptyLocalTrust() LocalTrust {
	return &localTrust{m: &mat.Dense{}}
}

// ReadLocalTrustFromCsv reads a local trust matrix from the given CSV file.
func ReadLocalTrustFromCsv(
	reader CsvReader, peerIndices map[string]int,
) (LocalTrust, error) {
	parseFields := func(fields []string) (
		from int, to int, level float64, err error,
	) {
		from, to = -1, -1
		if len(fields) < 2 {
			err = errors.New("too few fields")
		} else if from, err = ParsePeerId(fields[0],
			peerIndices); err != nil {
			err = errors.Wrapf(err, "invalid from %#v", fields[0])
		} else if to, err = ParsePeerId(fields[1], peerIndices); err != nil {
			err = errors.Wrapf(err, "invalid to %#v", fields[1])
		} else if len(fields) >= 3 {
			if level, err = ParseTrustLevel(fields[2]); err != nil {
				err = errors.Wrapf(err, "invalid trust level %#v", fields[2])
			}
		} else {
			level = 1.0
		}
		return
	}
	lt := NewEmptyLocalTrust()
	count := 0
	fields, err := reader.Read()
	for ; err == nil; fields, err = reader.Read() {
		count++
		from, to, level, err := parseFields(fields)
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot parse local trust CSV record #%d", count)
		}
		n := from
		if n < to {
			n = to
		}
		n++
		if lt.Dim() < n {
			lt = lt.Grow(n - lt.Dim())
		}
		lt.Set(from, to, level)
	}
	if err != io.EOF {
		return nil, errors.Wrapf(err,
			"cannot read local trust CSV record #%d", count+1)
	}
	return lt, nil
}
