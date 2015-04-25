package lzbase

import (
	"errors"
	"io"
)

// The maximum possible limit.
const maxLimit = 1<<63 - 1

// LimitedWriter wraps the writer and implements a limit for the number of
// bytes written. The field N should be set to MaxLimit if a practically
// unlimited writer is required.
type LimitedWriter struct {
	W io.Writer
	N int64
}

// Limit indicates that the write limit has been reached.
var Limit = errors.New("limit reached")

// Writes p bytes. The error value Limit is returned if the number N of bytes
// that can sill be written becomes zero or negative.
func (l *LimitedWriter) Write(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, Limit
	}
	if int64(len(p)) > l.N {
		err = Limit
		p = p[:l.N]
	}
	var werr error
	n, werr = l.W.Write(p)
	l.N -= int64(n)
	if werr != nil {
		return n, werr
	}
	return n, err
}
