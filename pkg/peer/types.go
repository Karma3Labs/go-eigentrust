package peer

import "fmt"

// Id is an opaque peer identifier.
type Id = string

// Index is a matrix/vector entry index used to identify the peer.
type Index = int

type NoSuchIndex struct {
	Value int
}

func (e NoSuchIndex) Error() string {
	return fmt.Sprintf("No such peer index %#v", e.Value)
}

type NoSuchId struct {
	Value Id
}

func (e NoSuchId) Error() string {
	return fmt.Sprintf("No such peer identifier %#v", e.Value)
}

type InvalidIndexLiteral struct {
	Value string
	Err   error
}

func (e InvalidIndexLiteral) Error() string {
	return fmt.Sprintf("Invalid peer index literal %#v: %v", e.Value, e.Err)
}

func (e InvalidIndexLiteral) Unwrap() error { return e.Err }

type NegativeIndex struct {
	Value Index
}

func (e NegativeIndex) Error() string {
	return fmt.Sprintf("negative peer index %#v", e.Value)
}
