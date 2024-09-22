package util

import (
	"errors"
	"fmt"
	"slices"
)

type CSVFieldExtractor struct {
	Indices []int
}

func NewCSVFieldExtractor(
	header []string, names ...string,
) (*CSVFieldExtractor, error) {
	indices := Map(names, func(name string) int {
		return slices.Index(header, name)
	})
	if x := slices.Index(indices, -1); x != -1 {
		return nil, fmt.Errorf("field %#v not in CSV header %#v",
			names[x], header)
	}
	return &CSVFieldExtractor{Indices: indices}, nil
}

func wrapOOB(fields []string, err error) error {
	if errors.Is(err, IndexOutOfBoundsError{}) {
		err = fmt.Errorf("too few fields in CSV record %#v: %w", fields, err)
	}
	return err
}

func (s *CSVFieldExtractor) ExtractAll(fields []string) ([]string, error) {
	extracted, err := MapWithErr(s.Indices,
		ElementAtWithErrFn(fields))
	return extracted, wrapOOB(fields, err)
}

func (s *CSVFieldExtractor) Extract(
	index int, fields []string,
) (string, error) {
	extracted, err := ElementAtWithErr(fields, index)
	return extracted, wrapOOB(fields, err)
}

// CSVReader reads from a CSV file.
type CSVReader interface {
	Read() (fields []string, err error)
}

// CSVWriter writes into a CSV file.
type CSVWriter interface {
	Write(fields []string) error
}
