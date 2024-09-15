package basic

import (
	"errors"
	"fmt"
	"io"

	"k3l.io/go-eigentrust/pkg/sparse"
)

// CanonicalizeTrustVector canonicalizes trustVector in-place,
// i.e. scales it so that the elements sum to one,
// or makes it a uniform vector that sums to one
// if the receiver is a zero vector.
func CanonicalizeTrustVector(v *sparse.Vector) {
	if errors.Is(Canonicalize(v.Entries), sparse.ErrZeroSum) {
		v.Entries = make([]sparse.Entry, v.Dim)
		c := 1 / float64(v.Dim)
		for i := range v.Entries {
			v.Entries[i].Index = i
			v.Entries[i].Value = c
		}
	}
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
			err = fmt.Errorf("invalid peer %#v: %w", fields[0], err)
		} else if len(fields) >= 2 {
			if level, err = ParseTrustLevel(fields[1]); err != nil {
				err = fmt.Errorf("invalid trust level %#v: %w", fields[1], err)
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
			return nil, fmt.Errorf(
				"cannot parse trust vector CSV record #%d: %w", count, err)
		}
		if maxPeer < peer {
			maxPeer = peer
		}
		entries = append(entries, sparse.Entry{Index: peer, Value: level})
	}
	if err != io.EOF {
		return nil, fmt.Errorf(
			"cannot read trust vector CSV record #%d: %w", count+1, err)
	}

	return sparse.NewVector(maxPeer+1, entries), nil
}
