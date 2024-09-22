package basic

import (
	"errors"

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
