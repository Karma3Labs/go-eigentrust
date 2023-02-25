package sparse

import "sort"

// Entry is an entry in a sparse vector or matrix.
type Entry struct {
	// Index is the index of the entry.
	// Context decides the meaning: For example, it is a column index when used
	// as a compressed sparse row (CSR) matrix.
	Index int

	// Value is the entry value.  For sparse use, it should be nonzero.
	Value float64
}

// CooEntry is a sparse matrix coordinate-format ("Coo") entry.
// Used as an input to a sparse matrix builder.
type CooEntry struct {
	Row, Column int
	Value       float64
}

// CSREntriesSort sorts CooEntry objects by (row, column) key.
type CSREntriesSort []CooEntry

func (a CSREntriesSort) Len() int      { return len(a) }
func (a CSREntriesSort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a CSREntriesSort) Less(i, j int) bool {
	switch {
	case a[i].Row < a[j].Row:
		return true
	case a[i].Row > a[j].Row:
		return false
	case a[i].Column < a[j].Column:
		return true
	case a[i].Column > a[j].Column:
		return false
	}
	return false
}

// CSCEntriesSort sorts CooEntry objects by (column, row) key.
type CSCEntriesSort []CooEntry

func (a CSCEntriesSort) Len() int      { return len(a) }
func (a CSCEntriesSort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a CSCEntriesSort) Less(i, j int) bool {
	switch {
	case a[i].Column < a[j].Column:
		return true
	case a[i].Column > a[j].Column:
		return false
	case a[i].Row < a[j].Row:
		return true
	case a[i].Row > a[j].Row:
		return false
	}
	return false
}

type entrySort []Entry

func (a entrySort) Len() int           { return len(a) }
func (a entrySort) Less(i, j int) bool { return a[i].Index < a[j].Index }
func (a entrySort) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func SortEntries(entries []Entry) []Entry {
	sort.Sort(entrySort(entries))
	return entries
}
