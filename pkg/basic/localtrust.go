package basic

import (
	"errors"
	"fmt"
	"io"

	"k3l.io/go-eigentrust/pkg/sparse"
)

// CanonicalizeLocalTrust canonicalizes localTrust in-place,
// i.e. scales each row so that its entries sum to one.
//
// If a non-nil preTrust vector is given,
// CanonicalizeLocalTrust substitutes it for zero rows in localTrust,
// i.e. the preTrust vector serves as the default outbound trust
// for peers without trust opinions.
//
// If preTrust is not nil, it must have the same dimension as localTrust.
func CanonicalizeLocalTrust(
	localTrust *sparse.Matrix, preTrust *sparse.Vector,
) error {
	n, err := localTrust.Dim()
	if err != nil {
		return err
	}
	if preTrust != nil && n != preTrust.Dim {
		return sparse.ErrDimensionMismatch
	}
	for i := 0; i < n; i++ {
		inRow := localTrust.RowVector(i)
		switch err := Canonicalize(inRow.Entries); err {
		case nil:
		case sparse.ErrZeroSum:
			if preTrust != nil {
				localTrust.SetRowVector(i, preTrust)
			}
		default:
			return err
		}
	}
	return nil
}

// ExtractDistrust extracts negative local trust from the given
// local trust, leaving only positive ones in the original.
// Extracted negative values are sign reversed, i.e. they are positive.
func ExtractDistrust(
	localTrust *sparse.Matrix,
) (*sparse.Matrix, error) {
	n, err := localTrust.Dim()
	if err != nil {
		return nil, err
	}
	distrust := sparse.NewCSRMatrix(n, n, nil)
	for truster := 0; truster < n; truster++ {
		trustRow := localTrust.Entries[truster]
		distrustRow := distrust.Entries[truster]
		for i, entry := range trustRow {
			if entry.Value >= 0 {
				trustRow[i-len(distrustRow)] = entry
			} else {
				entry.Value = -entry.Value
				distrustRow = append(distrustRow, entry)
			}
		}
		trustRow = trustRow[:len(trustRow)-len(distrustRow)]
		if len(trustRow) == 0 {
			trustRow = nil
		}
		localTrust.Entries[truster] = trustRow
		distrust.Entries[truster] = distrustRow
	}
	return distrust, nil
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
			err = fmt.Errorf("invalid from %#v: %w", fields[0], err)
		} else if to, err = ParsePeerId(fields[1], peerIndices); err != nil {
			err = fmt.Errorf("invalid to %#v: %w", fields[1], err)
		} else if len(fields) >= 3 {
			if level, err = ParseTrustLevel(fields[2]); err != nil {
				err = fmt.Errorf("invalid trust level %#v: %w", fields[2], err)
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
			return nil, fmt.Errorf(
				"cannot parse local trust CSV record #%d: %w", count, err)
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
		return nil, fmt.Errorf(
			"cannot read local trust CSV record #%d: %w", count+1, err)
	}
	maxIndex := maxFrom
	if maxIndex < maxTo {
		maxIndex = maxTo
	}
	dim := maxIndex + 1
	return sparse.NewCSRMatrix(dim, dim, entries), nil
}
