package lzma

import "io"

type Reader struct {
	Params Parameters
	StreamReader
}

// NewReader creates a new LZMA reader.
func NewReader(lzma io.Reader) (r *Reader, err error) {
	p, err := readHeader(lzma)
	if err != nil {
		return nil, err
	}
	p.normalizeReaderSizes()
	sr, err := NewStreamReader(lzma, *p)
	if err != nil {
		return nil, err
	}
	r = &Reader{
		Params:       *p,
		StreamReader: *sr,
	}
	return r, nil
}
