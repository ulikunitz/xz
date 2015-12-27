package lzma

import (
	"errors"
	"io"
)

// breader converts a reader into a byte reader.
type breader struct {
	io.Reader
	// helper slice
	p []byte
}

// ByteReader converts a reader into a ByteReader.
func ByteReader(r io.Reader) io.ByteReader {
	br, ok := r.(io.ByteReader)
	if !ok {
		return &breader{r, make([]byte, 1)}
	}
	return br
}

// ReadByte read byte function.
func (r *breader) ReadByte() (c byte, err error) {
	n, err := r.Reader.Read(r.p)
	if n < 1 {
		if err == nil {
			err = errors.New("ReadByte: no data")
		}
		return 0, err
	}
	return r.p[0], nil
}
