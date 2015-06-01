package lzma

import (
	"io"
)

// NewReader creates a new LZMA reader.
func NewReader(lzma io.Reader) (r *Reader, err error) {
	p, err := readHeader(lzma)
	if err != nil {
		return nil, err
	}
	p.normalizeReaderSizes()
	r, err = NewStreamReader(lzma, *p)
	return
}
