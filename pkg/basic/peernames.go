package basic

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"k3l.io/go-eigentrust/pkg/util"
)

func ReadPeerNamesFromCsv(reader util.CSVReader) (
	names []string, indices map[string]int, err error,
) {
	indices = map[string]int{}
	var fields []string
	for fields, err = reader.Read(); err == nil; fields, err = reader.Read() {
		index := len(names)
		if len(fields) < 1 {
			return nil, nil, fmt.Errorf("missing peer name (record #%d)", index)
		}
		name := fields[0]
		if existing, duplicate := indices[name]; duplicate {
			return nil, nil, fmt.Errorf("duplicate peer name (records #%d and #%d)",
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
