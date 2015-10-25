package lzma

import (
	"bufio"
	"io"
)

// Reader represents a reader for LZMA streams in the classic format.
type Reader struct {
	Parameters Parameters
	d          Decoder
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
		br = bufio.NewReader(lzma)
	}

	props, err := NewProperties(params.LC, params.LP, params.PB)
	if err != nil {
		return nil, err
	}
	state := NewState(props)

	dict, err := NewDecoderDict(params.DictCap, params.BufCap)
	if err != nil {
		return nil, err
	}

	r = &Reader{Parameters: *params}
	codecParams := CodecParams{
		Size:      params.Size,
		EOSMarker: params.EOSMarker,
	}
	if err = r.d.Init(br, state, dict, codecParams); err != nil {
		return nil, err
	}
	return r, nil
}

// Read reads data out of the LZMA reader.
func (r *Reader) Read(p []byte) (n int, err error) {
	return r.d.Read(p)
}
