// Package util contains various utilities used by go-eigentrust.
package util

import (
	"context"
	"fmt"
	"slices"
)

// GrowCap grows the capacity of a slice to at least the given target.
// The size grow exponentially, in order to avoid frequent reallocation/moving.
func GrowCap[T any](s []T, target int) []T {
	c := cap(s)
	for c < target {
		c = c*11/10 + 10
	}
	return slices.Grow(s, c-target)
}

// ShrinkWrap shrink-wraps the slice, i.e. leaves no excess capacity.
// Identical to slices.Clip, except it coerces zero-length slice into nil.
func ShrinkWrap[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	return slices.Clip(s)
}

// ElementAtWithErr returns the element at the given index.
func ElementAtWithErr[T any](s []T, i int) (elem T, err error) {
	if i < len(s) {
		elem = s[i]
	} else {
		err = IndexOutOfBoundsError{i, len(s)}
	}
	return
}

// ElementAtFn returns a function that returns the element at the given index.
func ElementAtFn[T any](s []T) func(int) T {
	return func(i int) T { return s[i] }
}

// ElementAtWithErrFn returns a function that returns the element at the given index.
func ElementAtWithErrFn[T any](s []T) func(int) (T, error) {
	return func(i int) (elem T, err error) { return ElementAtWithErr(s, i) }
}

// Map applies a function to each slice element and returns results as a slice.
func Map[S any, T any](s []S, f func(S) T) (t []T) {
	for _, v := range s {
		t = append(t, f(v))
	}
	return
}

// MapWithErr applies a function to each slice element
// and returns results as a slice, stopping at the first error return.
//
// len(t) < len(s) iff err != nil; in this case, err is from the function
// called with s[len(t)].
func MapWithErr[S any, T any](
	s []S, f func(S) (T, error),
) (t []T, err error) {
	t = make([]T, 0, len(s))
	var tv T
	for _, sv := range s {
		tv, err = f(sv)
		if err != nil {
			return
		}
		t = append(t, tv)
	}
	return
}

// MapErrRet maps the non-nil error return value onto another error
// using the given function.
func MapErrRet[T any](ret T, err error, f func(error) error) (T, error) {
	if err != nil {
		err = f(err)
	}
	return ret, err
}

// IndexOutOfBoundsError is returned when the requested index is out of bounds.
type IndexOutOfBoundsError struct {
	Index int
	Bound int
}

func (e IndexOutOfBoundsError) Error() string {
	return fmt.Sprintf("index %d out of bounds 0 <= index < %d", e.Index,
		e.Bound)
}

// SendElements sends all elements of the given slice to a channel.
func SendElements[T any](ctx context.Context, s []T, ch chan<- T) error {
	for _, v := range s {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- v:
		}
	}
	return nil
}

// ReceiveElements receives all elements of the given slice into a channel.
func ReceiveElements[T any](
	ctx context.Context, ch <-chan T,
) (s []T, err error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case v, ok := <-ch:
			if !ok {
				return s, nil
			}
			s = append(s, v)
		}
	}
}
