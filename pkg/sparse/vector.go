package sparse

import (
	"context"
	"math"
	"sort"
	"sync"
)

// Vector is a sparse vector.
type Vector struct {
	// Dim is the dimension of the vector.
	Dim int

	// Entries contain sparse entries, sorted by their Entry.Index.
	// For each Entry in Entries, 0 <= Entry.Index < Dim holds.
	Entries []Entry
}

// NNZ returns the number of non-zero entries.
func (v *Vector) NNZ() int {
	return len(v.Entries)
}

// NewVector creates and returns a new sparse vector with given entries.
func NewVector(dim int, entries []Entry) *Vector {
	return &Vector{
		Dim:     dim,
		Entries: SortEntriesByIndex(append(entries[:0:0], entries...)),
	}
}

// Assign clones (copies) the given vector into the receiver.
func (v *Vector) Assign(v1 *Vector) {
	v.Dim = v1.Dim
	v.Entries = append(v1.Entries[:0:0], v1.Entries...)
}

// Clone returns a copy (clone) of the receiver.
func (v *Vector) Clone() *Vector {
	return &Vector{
		Dim:     v.Dim,
		Entries: append(v.Entries[:0:0], v.Entries...),
	}
}

// SetDim grows/shrinks the receiver in-place to match the given dimension.
func (v *Vector) SetDim(dim int) {
	if dim < v.Dim {
		end := sort.Search(len(v.Entries),
			func(i int) bool { return v.Entries[i].Index >= dim })
		v.Entries = v.Entries[:end]
	}
	v.Dim = dim
}

// Sum computes the sum of all vector elements.
func (v *Vector) Sum() float64 {
	var summer KBNSummer
	for _, e := range v.Entries {
		summer.Add(e.Value)
	}
	return summer.Sum()
}

// AddVec stores v1 + v2 into the receiver.
func (v *Vector) AddVec(v1, v2 *Vector) error {
	if v1.Dim != v2.Dim {
		return ErrDimensionMismatch
	}
	e1, e2 := v1.Entries, v2.Entries
	var entries []Entry
	for len(e1) > 0 || len(e2) > 0 {
		if len(entries) == cap(entries) {
			newEntries := make([]Entry, 0, cap(entries)+max(len(e1), len(e2)))
			entries = append(newEntries, entries...)
		}
		var e Entry
		switch {
		case len(e1) == 0:
			e = e2[0]
			e2 = e2[1:]
		case len(e2) == 0:
			e = e1[0]
			e1 = e1[1:]
		case e1[0].Index < e2[0].Index:
			e = e1[0]
			e1 = e1[1:]
		case e2[0].Index < e1[0].Index:
			e = e2[0]
			e2 = e2[1:]
		default: // e1[0].Index == e2[0].Index
			e = Entry{Index: e1[0].Index, Value: e1[0].Value + e2[0].Value}
			e1, e2 = e1[1:], e2[1:]
		}
		entries = append(entries, e)
	}
	v.Dim = v1.Dim
	v.Entries = entries
	return nil
}

// SubVec stores v1 - v2 into the receiver.
func (v *Vector) SubVec(v1, v2 *Vector) error {
	if v1.Dim != v2.Dim {
		return ErrDimensionMismatch
	}
	e1, e2 := v1.Entries, v2.Entries
	var entries []Entry
	for len(e1) > 0 || len(e2) > 0 {
		if len(entries) == cap(entries) {
			newEntries := make([]Entry, 0, cap(entries)+max(len(e1), len(e2)))
			entries = append(newEntries, entries...)
		}
		var e Entry
		switch {
		case len(e1) == 0:
			e = Entry{Index: e2[0].Index, Value: -e2[0].Value}
			e2 = e2[1:]
		case len(e2) == 0:
			e = e1[0]
			e1 = e1[1:]
		case e1[0].Index < e2[0].Index:
			e = e1[0]
			e1 = e1[1:]
		case e2[0].Index < e1[0].Index:
			e = Entry{Index: e2[0].Index, Value: -e2[0].Value}
			e2 = e2[1:]
		default: // e1[0].Index == e2[0].Index
			e = Entry{Index: e1[0].Index, Value: e1[0].Value - e2[0].Value}
			e1, e2 = e1[1:], e2[1:]
		}
		entries = append(entries, e)
	}
	v.Dim = v1.Dim
	v.Entries = entries
	return nil
}

// ScaleVec scales the vector and stores the result into the receiver.
func (v *Vector) ScaleVec(a float64, v1 *Vector) {
	if a == 0 {
		v.Dim = v1.Dim
		v.Entries = nil
		return
	}
	if v1 != v {
		v.Assign(v1)
	}
	v.scaleInPlace(a)
}

func (v *Vector) scaleInPlace(a float64) {
	// The outer ScaleVec() covered a == 0.0 case.
	if a == 1 {
		return
	}
	zeros := 0
	for i := range v.Entries {
		v.Entries[i].Value *= a
		if v.Entries[i].Value == 0 {
			zeros++
		} else if zeros > 0 {
			v.Entries[i-zeros] = v.Entries[i]
		}
	}
	if zeros > 0 {
		// Zeros occur only in underflow and are considered rare;
		// we ignore the resulting extra capacity and don't shrink fit.
		v.Entries = v.Entries[:len(v.Entries)-zeros]
	}
}

// VecDot computes the dot product of the two given sparse vectors.
func VecDot(v1, v2 *Vector) float64 {
	n2 := len(v2.Entries)
	if n2 == 0 {
		return 0
	}
	i2, e2 := 0, v2.Entries[0]
	var summer KBNSummer
OverallLoop:
	for _, e1 := range v1.Entries {
		for e2.Index <= e1.Index {
			if e1.Index == e2.Index {
				value := e1.Value * e2.Value
				summer.Add(value)
			}
			i2 += 1
			if i2 == n2 {
				break OverallLoop
			}
			e2 = v2.Entries[i2]
		}
	}
	return summer.Sum()
}

// MulVec stores m multiplied by v1 into the receiver.
func (v *Vector) MulVec(
	ctx context.Context, m *Matrix, v1 *Vector,
) error {
	dim, err := m.Dim()
	if err != nil {
		return err
	}
	if dim != v1.Dim {
		return ErrDimensionMismatch
	}
	// Distribute row jobs onto workers, who publish to entries.
	// Workers feed entries chan; they exit upon jobs are exhausted.
	// entries chan is closed when all workers exit (no one left to feed it).
	jobs := make(chan int, dim)
	go func() {
		defer close(jobs)
		for row := 0; row < dim; row++ {
			select {
			case <-ctx.Done():
				return // closes jobs, which in turn terminates workers
			case jobs <- row:
			}
		}
	}()
	numWorkers := 32
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	entries := make(chan Entry, dim)
	for workerIndex := 0; workerIndex < numWorkers; workerIndex++ {
		go func(workerIndex int) {
			defer wg.Done()
			row, ok := 0, false
			for {
				select {
				case <-ctx.Done():
					return
				case row, ok = <-jobs:
					if !ok {
						return
					}
				}
				product := VecDot(m.RowVector(row), v1)
				select {
				case <-ctx.Done():
					return
				case entries <- Entry{Index: row, Value: product}:
				}
			}
		}(workerIndex)
	}
	go func() {
		wg.Wait()
		close(entries)
	}()
	var sortedEntries []Entry
Loop:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e, ok := <-entries:
			if !ok {
				break Loop
			}
			if e.Value != 0 {
				sortedEntries = append(sortedEntries, e)
			}
		}
	}
	sort.Sort(EntriesByIndex(sortedEntries))
	v.Dim = dim
	v.Entries = sortedEntries
	return nil
}

// Norm2 returns the Frobenius norm (sqrt of sum of elements).
func (v *Vector) Norm2() float64 {
	var summer KBNSummer
	for i := range v.Entries {
		value := v.Entries[i].Value
		summer.Add(value * value)
	}
	return math.Sqrt(summer.Sum())
}

// Merge merges the given vector (v2) into the receiver.
//
// If both v and v2 contain an entry at the same location, v2's entry wins.
//
// v2 is reset after merge.
func (v *Vector) Merge(v2 *Vector) {
	v.SetDim(max(v.Dim, v2.Dim)) // also resizes v.Entries
	v.Entries = mergeSpan(v.Entries, v2.Entries)
	v2.Reset()
}

// Reset resets the receiver to be empty (0x0).
func (v *Vector) Reset() {
	v.Dim = 0
	v.Entries = nil
}
