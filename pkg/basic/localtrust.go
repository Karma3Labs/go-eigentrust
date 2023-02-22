package basic

import (
	"io"

	"github.com/pkg/errors"
	"k3l.io/go-eigentrust/pkg/sparse"
)

// CanonicalizeLocalTrust canonicalizes localTrust in-place,
// i.e. scales each row so that its entries sum to one.
//
// Zero rows are replaced by preTrust vector.
//
// The receiver and trustVector must have the same dimension.
func CanonicalizeLocalTrust(
	localTrust *sparse.Matrix, preTrust *sparse.Vector,
) error {
	n := localTrust.Dim()
	if n != preTrust.Dim {
		return sparse.ErrDimensionMismatch
	}
	for i := 0; i < n; i++ {
		inRow := localTrust.RowVector(i)
		switch err := Canonicalize(inRow.Entries); err {
		case nil:
		case sparse.ErrZeroSum:
			localTrust.SetRowVector(i, preTrust)
		default:
			return err
		}
	}
	return nil
}

// GrowLocalTrust grows localTrust in-place by the specified amount.
// Newly added rows/columns are zero.
func GrowLocalTrust(localTrust *sparse.Matrix, by int) {
	if by < 0 {
		panic("negative by")
	}
	localTrust.MajorDim += by
	localTrust.MinorDim += by
	for i := 0; i < by; i++ {
		localTrust.Entries = append(localTrust.Entries, nil)
	}
}

// ReadLocalTrustFromCsv reads a local trust matrix from the given CSV file.
func ReadLocalTrustFromCsv(
	reader CsvReader, peerIndices map[string]int,
) (*sparse.Matrix, error) {
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
	count := 0
	fields, err := reader.Read()
	var entries []sparse.CooEntry
	maxFrom, maxTo := -1, -1
	for ; err == nil; fields, err = reader.Read() {
		count++
		from, to, level, err := parseFields(fields)
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot parse local trust CSV record #%d", count)
		}
		if maxFrom < from {
			maxFrom = from
		}
		if maxTo < to {
			maxTo = to
		}
		entries = append(entries, sparse.CooEntry{
			Row:    from,
			Column: to,
			Value:  level,
		})
	}
	if err != io.EOF {
		return nil, errors.Wrapf(err,
			"cannot read local trust CSV record #%d", count+1)
	}
	maxIndex := maxFrom
	if maxIndex < maxTo {
		maxIndex = maxTo
	}
	dim := maxIndex + 1
	return sparse.NewCSRMatrix(dim, dim, entries), nil
}
