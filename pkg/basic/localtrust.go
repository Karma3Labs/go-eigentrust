package basic

import (
	"errors"

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
		err := Canonicalize(inRow.Entries)
		switch {
		case err == nil:
		case errors.Is(err, sparse.ErrZeroSum):
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
	distrust := sparse.NewCSRMatrix(n, n, nil, false)
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
