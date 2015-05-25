package lzma

import (
	"io"

	"github.com/uli-go/xz/lzb"
)

// Reader supports the decoding of data in the classic LZMA format.
type Reader struct {
	io.Reader
	Parameters
}

// NewReader creates a new LZMA reader.
func NewReader(r io.Reader) (lr *Reader, err error) {
	p, err := readHeader(r)
	if err != nil {
		return nil, err
	}
	if err = verifyParameters(p); err != nil {
		return nil, err
	}
	lzbParams := lzb.Params{
		Properties: p.Properties(),
		DictSize:   p.DictSize,
		BufferSize: p.BufferSize,
	}
	lzbR, err := lzb.NewReader(r, lzbParams)
	if err != nil {
		return nil, err
	}
	if !p.SizeInHeader {
		return &Reader{lzbR, *p}, nil
	}
	panic("TODO")
}
