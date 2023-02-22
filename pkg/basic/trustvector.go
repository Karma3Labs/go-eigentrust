package basic

import (
	"io"

	"github.com/pkg/errors"
	"k3l.io/go-eigentrust/pkg/sparse"
)

// CanonicalizeTrustVector canonicalizes trustVector in-place,
// i.e. scales it so that the elements sum to one,
// or makes it a uniform vector that sums to one
// if the receiver is a zero vector.
func CanonicalizeTrustVector(v *sparse.Vector) {
	if Canonicalize(v.Entries) == sparse.ErrZeroSum {
		v.Entries = make([]sparse.Entry, v.Dim)
		c := 1 / float64(v.Dim)
		for i := range v.Entries {
			v.Entries[i].Index = i
			v.Entries[i].Value = c
		}
	}
}

// GrowTrustVector grows localTrust in-place by the specified amount.
// Newly added rows/columns are zero.
func GrowTrustVector(trustVector *sparse.Vector, by int) {
	if by < 0 {
		panic("negative by")
	}
	trustVector.Dim += by
}

// ReadTrustVectorFromCsv reads a trust vector from the given CSV file.
func ReadTrustVectorFromCsv(
	reader CsvReader, peerIndices map[string]int,
) (*sparse.Vector, error) {
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
	count := 0
	fields, err := reader.Read()
	maxPeer := -1
	var entries []sparse.Entry
	for ; err == nil; fields, err = reader.Read() {
		count++
		peer, level, err := parseFields(fields)
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot parse trust vector CSV record #%d", count)
		}
		if maxPeer < peer {
			maxPeer = peer
		}
		entries = append(entries, sparse.Entry{Index: peer, Value: level})
	}
	if err != io.EOF {
		return nil, errors.Wrapf(err,
			"cannot read trust vector CSV record #%d", count+1)
	}

	return sparse.NewVector(maxPeer+1, entries), nil
}
