package util

import (
	"context"
	"errors"
)

// Must takes a function call's return tuple, whose last element is an error,
// and panics if the error is not nil.  Upon no error, it returns the tuple
// minus the error element at the end.
func Must[R any](r R, err error) R {
	if err != nil {
		panic(err)
	}
	return r
}

// ErrFromCh expects an error from the given channel and returns it.
// If ch is closed, ErrFromCh returns an error.
func ErrFromCh(ctx context.Context, ch <-chan error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err, ok := <-ch:
		if !ok {
			err = errors.New("error channel closed unexpectedly")
		}
		return err
	}
}
