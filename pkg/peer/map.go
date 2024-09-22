package peer

import (
	"bufio"
	"context"
	"io"
	"maps"
	"os"
	"slices"

	"k3l.io/go-eigentrust/pkg/util"
)

// Map is a bidirectional peer map between string identifier and integer index,
// with support for allocation.
type Map struct {
	indices map[Id]Index
	ids     []Id
}

// NewMap returns a new, empty peer map.
func NewMap() *Map {
	return &Map{indices: make(map[Id]Index)}
}

// MapWithIds returns a new map initialized with the given peer identifiers,
// sequentially assigning peer indices to them.
func MapWithIds(ids ...Id) *Map {
	m := NewMap()
	for _, id := range ids {
		_ = m.Allocate(id)
	}
	return m
}

// MapWithIdChan returns a new map initialized with the peer identifiers
// taken from the given channel, sequentially assigning peer indices to them.
func MapWithIdChan(ctx context.Context, ch <-chan Id) (*Map, error) {
	m := NewMap()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case id, ok := <-ch:
			if !ok {
				return m, nil
			}
			_ = m.Allocate(id)
		}
	}
}

// MapWithIdScanner returns a new map initialized with the peer identifiers
// taken from the given scanner, sequentially assigning peer indices to them.
func MapWithIdScanner(scanner *bufio.Scanner) (*Map, error) {
	m := NewMap()
	for scanner.Scan() {
		_ = m.Allocate(scanner.Text())
	}
	return m, scanner.Err()
}

// MapWithIdReader returns a new map initialized with the peer identifiers
// taken from the given plaintext reader, one per line,
// sequentially assigning peer indices to them.
func MapWithIdReader(r io.Reader) (*Map, error) {
	return MapWithIdScanner(bufio.NewScanner(r))
}

// MapWithIdFile returns a new map initialized with the peer identifiers
// taken from the given file, one per line,
// sequentially assigning peer indices to them.
func MapWithIdFile(path string) (*Map, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)
	return MapWithIdReader(f)
}

// Allocate returns the peer index for the given id, allocating one if needed.
func (m *Map) Allocate(id Id) (index Index) {
	index, ok := m.indices[id]
	if !ok {
		index = len(m.ids)
		m.ids = append(m.ids, id)
		m.indices[id] = index
	}
	return
}

// Index returns the peer index for the given id.
func (m *Map) Index(id Id) (index Index, ok bool) {
	index, ok = m.indices[id]
	return
}

// Id returns the peer id for the given index.
func (m *Map) Id(index Index) (id Id, ok bool) {
	if ok = index < len(m.ids); ok {
		id = m.ids[index]
	}
	return
}

// Clear clears the map.  Subsequent allocation starts from index 0 again.
func (m *Map) Clear() {
	clear(m.indices)
	m.ids = nil
}

// Ids returns (a copy of) the index-to-identifier array.
func (m *Map) Ids() []Id {
	return slices.Clone(m.ids)
}

// Indices returns (a copy of) the identifier-to-index map.
func (m *Map) Indices() map[Id]Index {
	return maps.Clone(m.indices)
}

// Len returns the number of peers.
func (m *Map) Len() int { return len(m.ids) }
