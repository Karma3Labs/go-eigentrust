package util

import (
	"io"
	"os"
)

type DummyWriteCloser struct{}

func (wc DummyWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (wc DummyWriteCloser) Close() error                { return nil }

type WriteNoCloser struct{ io.Writer }

func (w WriteNoCloser) Close() error { return nil }

// OpenOutputFile opens and returns a file for output.
// If filename is "", it returns a dummy WriteCloser that does nothing.
// If filename is "-"/"!", it returns a stdout/stderr; its Close() does nothing.
func OpenOutputFile(filename string) (io.WriteCloser, error) {
	switch filename {
	case "":
		return DummyWriteCloser{}, nil
	case "-":
		return WriteNoCloser{os.Stdout}, nil
	case "!":
		return WriteNoCloser{os.Stderr}, nil
	default:
		return os.Create(filename)
	}
}

// Close tries to close a closer, ignoring any error.
// For use with defer.
func Close(c io.Closer) { _ = c.Close() }
