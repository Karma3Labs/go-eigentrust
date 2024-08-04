package sparse

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"syscall"
	"unsafe"

	"github.com/rs/zerolog"
)

// CSMatrix is a compressed sparse matrix.
// Used as the base of CSRMatrix and CSCMatrix.
//
// (Shallow-)copying CSMatrix is lightweight.
type CSMatrix struct {
	MajorDim, MinorDim int
	Entries            [][]Entry
	mapped             []byte
}

// Reset resets the receiver to be empty (0x0).
func (m *CSMatrix) Reset() {
	err := m.Munmap()
	if err != nil {
		panic(err)
	}
	m.MajorDim = 0
	m.MinorDim = 0
	m.Entries = nil
}

// Dim asserts the receiver is a square matrix and returns the dimension.
func (m *CSMatrix) Dim() (int, error) {
	if m.MajorDim != m.MinorDim {
		return 0, ErrDimensionMismatch
	}
	return m.MajorDim, nil
}

// SetMajorDim grows/shrinks the receiver in-place,
// so it matches the given major dimension.
func (m *CSMatrix) SetMajorDim(dim int) {
	if cap(m.Entries) < dim {
		m.Entries = append(make([][]Entry, 0, dim), m.Entries...)
	}
	m.Entries = m.Entries[:dim]
	m.MajorDim = dim
}

// SetMinorDim grows/shrinks the receiver in-place,
// so it matches the given minor dimension.
func (m *CSMatrix) SetMinorDim(dim int) {
	if dim < m.MinorDim {
		for maj, entries := range m.Entries {
			end := sort.Search(len(entries),
				func(i int) bool { return entries[i].Index >= dim })
			m.Entries[maj] = entries[:end]
		}
	}
	m.MinorDim = dim
}

// NNZ counts nonzero entries.
func (m *CSMatrix) NNZ() (nnz int) {
	for _, row := range m.Entries {
		nnz += len(row)
	}
	return
}

// Transpose transposes the sparse matrix.
func (m *CSMatrix) Transpose(ctx context.Context) (*CSMatrix, error) {
	nnzs := make([]int, m.MinorDim) // indexed by column
	for _, rowEntries := range m.Entries {
		for _, e := range rowEntries {
			nnzs[e.Index]++
		}
	}
	transposedEntries := make([][]Entry, m.MinorDim)
	for col, nnz := range nnzs {
		if nnz != 0 {
			transposedEntries[col] = make([]Entry, 0, nnz)
		}
	}
	for row, rowEntries := range m.Entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		for _, e := range rowEntries {
			col := e.Index
			transposedEntries[col] = append(transposedEntries[col],
				Entry{Index: row, Value: e.Value})
		}
	}
	mt := &CSMatrix{
		MajorDim: m.MinorDim,
		MinorDim: m.MajorDim,
		Entries:  transposedEntries,
	}
	runtime.SetFinalizer(mt, (*CSMatrix).finalize)
	return mt, nil
}

func mergeSpan(s1, s2 []Entry) []Entry {
	switch {
	case len(s1) == 0 && len(s2) == 0:
		return nil
	case len(s1) == 0:
		return s2
	case len(s2) == 0:
		return s1
	}
	s := make([]Entry, 0, len(s1)+len(s2)) // worst case, shrink-wrap later
	i1, i2 := 0, 0
	var more1, more2 bool
	updateMore := func() { more1, more2 = i1 < len(s1), i2 < len(s2) }
	for updateMore(); more1 || more2; updateMore() {
		switch {
		case !more2:
			s = append(s, s1[i1])
			i1++
		case !more1:
			s = append(s, s2[i2])
			i2++
		default:
			index1, index2 := s1[i1].Index, s2[i2].Index
			switch {
			case index1 < index2:
				s = append(s, s1[i1])
				i1++
			case index2 < index1:
				s = append(s, s2[i2])
				i2++
			default: // s1[i1].Index == s2[i2].Index, s2 wins
				s = append(s, s2[i2])
				i1++
				i2++
			}
		}
	}
	return append(s[:0:0], s...) // shrink-wrap
}

// Merge merges the given matrix (m2) into the receiver.
//
// If both m and m2 contain an entry at the same location, m2's entry wins.
//
// m2 is reset after merge.
func (m *CSMatrix) Merge(m2 *CSMatrix) {
	// Load m2 back into memory, we are about to merge/reuse its spans in m.
	_ = m2.Munmap()

	m.SetMajorDim(max(m.MajorDim, m2.MajorDim)) // also resizes m.Entries
	m.SetMinorDim(max(m.MinorDim, m2.MinorDim))
	for i := 0; i < m2.MajorDim; i++ {
		m.Entries[i] = mergeSpan(m.Entries[i], m2.Entries[i])
	}
	m2.Reset()
}

// Mmap swaps out contents onto a temp file and mmap-s it, freeing core memory.
//
// If the receiver (m) is Mmap()-ed,
// future operations on it that replaces its major spans
// will not automatically map the replaced major spans.
// If needed, Mmap() can be called again on the same receiver
// in order to refresh the mapping
// and ensure that the entirety of the receiver's contents is swapped out.
// (In this case, Munmap() need not be called first.)
func (m *CSMatrix) Mmap(ctx context.Context) error {
	logger := zerolog.Ctx(ctx).
		With().Str("func", "sparse.(*CSMatrix).Mmap").Logger()
	if m.mapped != nil {
		nnz := uintptr(m.NNZ())
		start := uintptr(unsafe.Pointer(&m.mapped[0]))
		end := start + nnz*unsafe.Sizeof(Entry{})
		dirty := false
		for _, span := range m.Entries {
			if len(span) > 0 {
				spanStart := uintptr(unsafe.Pointer(unsafe.SliceData(span)))
				spanLen := uintptr(len(span))
				spanEnd := spanStart + spanLen*unsafe.Sizeof(Entry{})
				if spanStart < start || spanEnd > end {
					dirty = true
					break
				}
			}
		}
		if !dirty {
			logger.Debug().Msg("already mapped")
			return nil
		}
		logger.Debug().Msg("mapped but dirty, reloading")
	}
	nnz := m.NNZ()
	if int(uintptr(nnz)) != nnz {
		return fmt.Errorf("matrix too big (%#v entries)", nnz)
	}
	size := unsafe.Sizeof(Entry{}) * uintptr(nnz)
	if uintptr(int(size)) != size || int64(size) < 0 {
		return fmt.Errorf("matrix data too big (%#v bytes)", size)
	}
	tmpdir := os.Getenv("TMPDIR")
	if tmpdir == "" {
		tmpdir = "/tmp"
	}
	file, err := os.CreateTemp(tmpdir, "eigentrust-server-csmatrix.")
	if err != nil {
		return err
	}
	filename := file.Name()
	logger = logger.With().Str("filename", filename).Logger()
	logger.Debug().Int("nnz", nnz).Msg("swapping out")
	defer func() {
		if file != nil {
			logger.Trace().Msg("closing file without mapping")
			_ = file.Close()
		}
	}()
	logger.Trace().Uint64("size", uint64(size)).Msg("truncating")
	err = file.Truncate(int64(size))
	if err != nil {
		return err
	}
	logger.Trace().Msg("mmap-ing")
	mapped, err := syscall.Mmap(int(file.Fd()), 0, int(size),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return err
	}
	defer func() {
		if mapped != nil {
			logger.Trace().Msg("unmapping upon failure")
			_ = syscall.Munmap(mapped)
		}
	}()
	logger.Trace().Msg("closing after mapping")
	err = file.Close()
	if err != nil {
		logger.Err(err).Msg("cannot close file")
	}
	file = nil
	logger.Trace().Msg("removing file")
	err = os.Remove(filename)
	if err != nil {
		return err
	}
	e0 := (*Entry)(unsafe.Pointer(&mapped[0]))
	entries := unsafe.Slice(e0, nnz)
	logger.Trace().Msg("copying")
	var start int
	for major := range m.Entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		stride := len(m.Entries[major])
		span := entries[start : start+stride]
		copy(span, m.Entries[major])
		start += stride
	}
	if start != nnz {
		panic(fmt.Sprintf("size mismatch: start %#v != nnz %#v", start, nnz))
	}
	logger.Trace().Msg("finishing")
	if m.mapped != nil {
		// we already copied
		err := syscall.Munmap(m.mapped)
		if err != nil {
			logger.Err(err).Msg("cannot Munmap() old mapping while remapping")
		}
	}
	m.mapped = mapped
	mapped = nil
	logger.Trace().Msg("done")
	return nil
}

func (m *CSMatrix) Munmap() error {
	if m.mapped == nil {
		return nil
	}
	entries := make([][]Entry, 0, len(m.Entries))
	for _, span := range m.Entries {
		spanCopy := append(span[:0:0], span...)
		entries = append(entries, spanCopy)
	}
	err := syscall.Munmap(m.mapped)
	if err != nil {
		return err
	}
	m.Entries = entries
	m.mapped = nil
	return nil
}

func (m *CSMatrix) finalize() {
	_ = m.Munmap()
	logger := zerolog.New(os.Stderr).
		With().Str("m", fmt.Sprintf("%p", m)).Logger()
	logger.Trace().Msg("finalizing")
	if m.mapped != nil {
		err := syscall.Munmap(m.mapped)
		if err != nil {
			logger.Err(err).Msg("cannot unmap backing store")
		}
	}
}

// CSRMatrix is a compressed sparse row matrix.
type CSRMatrix struct {
	CSMatrix
}

// NewCSRMatrix creates a new compressed sparse row matrix
// with the given dimension and entries.
//
// The given entries are sorted in row-column order.
func NewCSRMatrix(
	rows, cols int,
	entries []CooEntry,
) *CSRMatrix {
	var entries2 [][]Entry
	if rows != 0 {
		entries2 = make([][]Entry, rows)
	}
	for _, e := range entries {
		if e.Value == 0 {
			continue
		}
		entries2[e.Row] = append(entries2[e.Row], Entry{
			Index: e.Column,
			Value: e.Value,
		})
	}
	for _, row := range entries2 {
		sort.Sort(EntriesByIndex(row))
	}
	m := &CSRMatrix{
		CSMatrix{
			MajorDim: rows,
			MinorDim: cols,
			Entries:  entries2,
		},
	}
	runtime.SetFinalizer(&m.CSMatrix, (*CSMatrix).finalize)
	return m
}

// Dims returns the numbers of rows/columns.
func (m *CSRMatrix) Dims() (rows, cols int) { return m.MajorDim, m.MinorDim }

// SetDim grows/shrinks the receiver in-place,
// so it contains the specified number of rows/columns.
func (m *CSRMatrix) SetDim(rows, cols int) {
	m.SetMajorDim(rows)
	m.SetMinorDim(cols)
}

// RowVector returns the given row as a sparse vector.
// The returned vector shares the same slice of entry objects.
func (m *CSRMatrix) RowVector(index int) *Vector {
	return &Vector{
		Dim:     m.MinorDim,
		Entries: m.Entries[index],
	}
}

// SetRowVector replaces the given row.
// The receiver shares the same slice of entry objects.
func (m *CSRMatrix) SetRowVector(index int, vector *Vector) {
	m.Entries[index] = vector.Entries
}

// Transpose transposes the matrix.
func (m *CSRMatrix) Transpose(ctx context.Context) (*CSRMatrix, error) {
	mt, err := m.CSMatrix.Transpose(ctx)
	switch err {
	case nil:
		return &CSRMatrix{*mt}, nil
	default:
		return nil, err
	}
}

// TransposeToCSC transposes the matrix.
// The returned matrix shares the same entry objects.
func (m *CSRMatrix) TransposeToCSC() *CSCMatrix {
	return &CSCMatrix{
		CSMatrix{
			MajorDim: m.MinorDim,
			MinorDim: m.MajorDim,
			Entries:  m.Entries,
		},
	}
}

// CSCMatrix is a compressed sparse column matrix.
type CSCMatrix struct {
	CSMatrix
}

// Dims returns the numbers of rows/columns.
func (m *CSCMatrix) Dims() (rows, cols int) { return m.MinorDim, m.MajorDim }

// SetDim grows/shrinks the receiver in-place,
// so it contains the specified number of rows/columns.
func (m *CSCMatrix) SetDim(rows, cols int) {
	m.SetMajorDim(cols)
	m.SetMinorDim(rows)
}

// ColumnVector returns the given row as a sparse vector.
// The returned vector shares the same entry objects.
func (m *CSCMatrix) ColumnVector(index int) *Vector {
	return &Vector{
		Dim:     m.MinorDim,
		Entries: m.Entries[index],
	}
}

// Transpose transposes the matrix.
func (m *CSCMatrix) Transpose(ctx context.Context) (*CSCMatrix, error) {
	mt, err := m.CSMatrix.Transpose(ctx)
	switch err {
	case nil:
		return &CSCMatrix{*mt}, nil
	default:
		return nil, err
	}
}

// TransposeToCSR transposes the matrix.
// The returned matrix shares the same entry objects.
func (m *CSCMatrix) TransposeToCSR() *CSRMatrix {
	return &CSRMatrix{
		CSMatrix{
			MajorDim: m.MinorDim,
			MinorDim: m.MajorDim,
			Entries:  m.Entries,
		},
	}
}

// Matrix is just an alias of CSRMatrix,
// which is the more popular of the two compressed sparse variants.
type Matrix = CSRMatrix
