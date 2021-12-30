package lzma

import (
	"errors"
	"io"
)

// limitedReader provides a reader with a limit on the data to read. We are
// using our own implementation because io.LimitReader is not a ByteReader,
// which we need for our implementation.
type limitedReader struct {
	r io.Reader
	n int64
}

// Read implements the normal read function on the reader.
func (l *limitedReader) Read(p []byte) (n int, err error) {
	if l.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.n {
		p = p[0:l.n]
	}
	n, err = l.r.Read(p)
	l.n -= int64(n)
	return n, err
}

// ReadByte reads a single byte from the reader.
func (l *limitedReader) ReadByte() (c byte, err error) {
	if l.n <= 0 {
		return 0, io.EOF
	}
	var a [1]byte
	n, err := l.r.Read(a[0:1])
	if n == 0 {
		if err == nil {
			err = errors.New("lzma: no data")
		}
		return 0, err
	}
	l.n--
	return a[0], nil
}
