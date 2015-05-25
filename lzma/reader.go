package lzma

import (
	"io"

	"github.com/uli-go/xz/lzb"
)

// Reader supports the decoding of data in the classic LZMA format.
type Reader struct {
	io.Reader
	lzb.Parameters
}

// NewReader creates a new LZMA reader.
func NewReader(r io.Reader) (lr *Reader, err error) {
	p, err := readHeader(r)
	if err != nil {
		return nil, err
	}
	if err = p.Verify(); err != nil {
		return nil, err
	}
	lzbR, err := lzb.NewReader(r, *p)
	if err != nil {
		return nil, err
	}
	if !p.SizeInHeader {
		return &Reader{lzbR, *p}, nil
	}
	panic("TODO")
}
