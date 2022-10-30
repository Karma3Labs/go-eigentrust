package basic

import (
	"io"
	"strconv"

	"github.com/pkg/errors"
)

func ReadPeerNamesFromCsv(reader CsvReader) (
	names []string, indices map[string]int, err error,
) {
	indices = map[string]int{}
	var fields []string
	for fields, err = reader.Read(); err == nil; fields, err = reader.Read() {
		index := len(names)
		if len(fields) < 1 {
			return nil, nil, errors.Errorf("missing peer name (record #%d)",
				index)
		}
		name := fields[0]
		if existing, duplicate := indices[name]; duplicate {
			return nil, nil, errors.Errorf("duplicate peer name (records #%d and #%d)",
				existing, index)
		}
		names = append(names, name)
		indices[name] = index
	}
	if err != io.EOF {
		return nil, nil, err
	}
	return names, indices, nil
}

// ParsePeerId parses s and returns the peer index.
//
// If peerIndices is not nil, s is considered to be a peer name found therein;
// otherwise, s is considered to be a peer index integer literal.
func ParsePeerId(s string, peerIndices map[string]int) (index int, err error) {
	if peerIndices != nil {
		var found bool
		if index, found = peerIndices[s]; !found {
			err = errors.New("unknown peer name")
		}
	} else {
		if index, err = strconv.Atoi(s); err != nil {
			err = errors.Wrap(err, "invalid peer index literal")
		} else if index < 0 {
			err = errors.New("negative peer index")
		}
	}
	return
}

// ParseTrustLevel parses s as a non-negative trust level and returns it.
func ParseTrustLevel(s string) (level float64, err error) {
	level, err = strconv.ParseFloat(s, 64)
	if err == nil {
		// Just return err as is
	} else if level < 0 {
		err = errors.New("negative trust level")
	}
	return
}
