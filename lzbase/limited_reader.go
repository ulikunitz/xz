package lzbase

import "io"

// LimitedReader wraps the Reader and verifies the EOF condition if N is zero.
// Note that EOF might be received before N has been counted down.
type LimitedReader struct {
	R *Reader
	N int64
}

// confirmEOF checks whether the EOF condition is supported by the wrapped
// Reader.
func (l *LimitedReader) confirmEOF() bool {
	// check for empty buffer
	if l.R.dict.readable() != 0 {
		return false
	}
	// check for cleared range decoder
	return l.R.rd.possiblyAtEnd()
}

// errWrongLimit indicates that the limit has been reached at a position where
// more data is available.
var errWrongLimit = newError("limit stops at wrong position")

// Read checks whether the limit has been reached and reads the requested
// bytes from the wrapped Reader. If the limit has been reached the functions
// confirms the EOF.
func (l *LimitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		if !l.confirmEOF() {
			return 0, errWrongLimit
		}
		return 0, io.EOF
	}
	if int64(len(p)) > l.N {
		p = p[:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return
}
