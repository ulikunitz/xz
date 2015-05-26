package lzma

import (
	"io"

	"github.com/uli-go/xz/lzb"
)

// NewReader creates a new LZMA reader.
func NewReader(r io.Reader) (lr io.Reader, err error) {
	p, err := readHeader(r)
	if err != nil {
		return nil, err
	}
	p.NormalizeSizes()
	if err = p.Verify(); err != nil {
		return nil, err
	}
	lr, err = lzb.NewReader(r, *p)
	return
}
