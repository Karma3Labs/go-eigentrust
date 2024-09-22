package sparse

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"

	"k3l.io/go-eigentrust/pkg/peer"
	spopt "k3l.io/go-eigentrust/pkg/sparse/option"
	"k3l.io/go-eigentrust/pkg/util"
)

// Entry is an entry in a sparse vector or matrix.
type Entry struct {
	// Index is the index of the entry.
	// Context decides the meaning: For example, it is a column index when used
	// as a compressed sparse row (CSR) matrix.
	Index int

	// Value is the entry value.  For sparse use, it should be nonzero.
	Value float64
}

func SendEntriesFromCSV(
	ctx context.Context, r util.CSVReader, ch chan<- Entry,
	opts ...spopt.Option,
) error {
	o := spopt.New(opts...)
	header, err := r.Read()
	if err != nil {
		return err
	}
	xt, err := util.NewCSVFieldExtractor(header, o.Row.Name, o.Value.Name)
	if err != nil {
		return err
	}
	for fields, err := r.Read(); err != io.EOF; fields, err = r.Read() {
		if err != nil {
			return err
		}
		iv, err := xt.ExtractAll(fields)
		if err != nil {
			return err
		}
		index, err := peer.ParseId(iv[0], o.Row.PeerMap, o.Row.Alloc)
		if err != nil {
			return fmt.Errorf("invalid peer id %#v: %w", iv[0], err)
		}
		value, err := strconv.ParseFloat(iv[1], 64)
		if err == nil && value < 0 && !o.Value.AllowNegative {
			err = NegativeValueError{value}
		}
		if err != nil {
			return fmt.Errorf("invalid value %#v: %w", iv[1], err)
		}
		coo := Entry{index, value}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- coo:
		}
	}
	return nil
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

type EntriesByIndex []Entry

func (a EntriesByIndex) Len() int           { return len(a) }
func (a EntriesByIndex) Less(i, j int) bool { return a[i].Index < a[j].Index }
func (a EntriesByIndex) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func SortEntriesByIndex(entries []Entry) []Entry {
	sort.Sort(EntriesByIndex(entries))
	return entries
}

type EntriesByValue []Entry

func (a EntriesByValue) Len() int           { return len(a) }
func (a EntriesByValue) Less(i, j int) bool { return a[i].Value < a[j].Value }
func (a EntriesByValue) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func SortEntriesByValue(entries []Entry) []Entry {
	sort.Sort(EntriesByValue(entries))
	return entries
}

func SendCooEntriesFromCSV(
	ctx context.Context, r util.CSVReader, ch chan<- CooEntry,
	opts ...spopt.Option,
) error {
	o := spopt.New(opts...)
	header, err := r.Read()
	if err != nil {
		return err
	}
	xt, err := util.NewCSVFieldExtractor(header,
		o.Row.Name, o.Column.Name, o.Value.Name)
	if err != nil {
		return err
	}
	for fields, err := r.Read(); err != io.EOF; fields, err = r.Read() {
		if err != nil {
			return err
		}
		ijv, err := xt.ExtractAll(fields)
		if err != nil {
			return err
		}
		row, err := peer.ParseId(ijv[0], o.Row.PeerMap, o.Row.Alloc)
		if err != nil {
			return fmt.Errorf("invalid row id %#v: %w", ijv[0], err)
		}
		column, err := peer.ParseId(ijv[1], o.Column.PeerMap, o.Column.Alloc)
		if err != nil {
			return fmt.Errorf("invalid column id %#v: %w", ijv[1], err)
		}
		value, err := strconv.ParseFloat(ijv[2], 64)
		if err == nil && value < 0 && !o.Value.AllowNegative {
			err = NegativeValueError{value}
		}
		if err != nil {
			return fmt.Errorf("invalid value %#v: %w", ijv[2], err)
		}
		coo := CooEntry{row, column, value}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- coo:
		}
	}
	return nil
}
