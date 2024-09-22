package peer

import (
	"strconv"
)

// ParseId parses the given peer identifier and returns a peer index.
//
// If m is not nil, id is considered to be a peer name found therein;
// otherwise, s is considered to be a peer index integer literal.
// If alloc is true, missing peer identifiers are allocated new indices in m;
// otherwise, missing peer identifiers are reported as an error.
func ParseId(id Id, m *Map, alloc bool) (Index, error) {
	if m != nil {
		if alloc {
			return m.Allocate(id), nil
		}
		index, found := m.Index(id)
		if !found {
			return 0, NoSuchId{id}
		}
		return index, nil
	} else {
		index, err := strconv.Atoi(id)
		if err != nil {
			return 0, InvalidIndexLiteral{id, err}
		}
		if index < 0 {
			return 0, NegativeIndex{index}
		}
		return index, nil
	}
}

// GetId turns the given peer index back into a peer identifier.
func GetId(index int, m *Map) (id Id, err error) {
	if m != nil {
		var ok bool
		if id, ok = m.Id(index); !ok {
			err = NoSuchIndex{index}
		}
	} else {
		id = strconv.Itoa(index)
	}
	return
}
