package grpcserver

import (
	"math"
	"math/big"
)

var maxBigUint64 = new(big.Int).SetUint64(math.MaxUint64)

func BigUint2Qwords(value *big.Int) (qwords []uint64) {
	count := (value.BitLen() + 63) / 64
	qwords = make([]uint64, count)
	v := new(big.Int).Set(value)
	for count > 0 {
		count--
		qwords[count] = new(big.Int).And(v, maxBigUint64).Uint64()
		v.Rsh(v, 64)
	}
	return qwords
}

func Qwords2BigUint(qwords []uint64) (v *big.Int) {
	v = new(big.Int)
	for _, w := range qwords {
		v.Lsh(v, 64)
		v.Or(v, new(big.Int).SetUint64(w))
	}
	return v
}
