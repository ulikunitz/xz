package lzma

import (
	"errors"
	"io"
)

// Reader represents a reader for LZMA streams in the classic format.
type Reader struct {
	Parameters Parameters
	d          *Decoder
}

// breader converts a reader into a byte reader.
type breader struct {
	io.Reader
}

// ReadByte read byte function.
func (r breader) ReadByte() (c byte, err error) {
	var p [1]byte
	n, err := r.Reader.Read(p[:])
	if n < 1 {
		if err == nil {
			err = errors.New("ReadByte: no data")
		}
		return 0, err
	}
	return p[0], nil
}

// NewReader creates a new reader for an LZMA stream using the classic
// format.
func NewReader(lzma io.Reader) (r *Reader, err error) {
	params, err := readHeader(lzma)
	if err != nil {
		return nil, err
	}
	params.normalizeReader()

	br, ok := lzma.(io.ByteReader)
	if !ok {
		br = breader{lzma}
	}

	state := NewState(params.Properties)

	dict, err := NewDecoderDict(params.DictCap, params.BufSize)
	if err != nil {
		return nil, err
	}

	r = &Reader{Parameters: *params}

	if r.d, err = NewDecoder(br, state, dict, params.Size); err != nil {
		return nil, err
	}

	return r, nil
}

// Read reads data out of the LZMA reader.
func (r *Reader) Read(p []byte) (n int, err error) {
	return r.d.Read(p)
}
