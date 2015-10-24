package lzma

/*

import "io"

// Reader represents a reader for LZMA streams in the classic format.
type Reader struct {
	d Decoder
}

// NewReader creates a new reader for an LZMA stream using the classic
// format.
func NewReader(lzma io.Reader) (r *Reader, err error) {
	params, err := readHeader(lzma)
	if err != nil {
		return nil, err
	}
	if params.DictCap < MinDictCap {
		params.DictCap = MinDictCap
	}
	params.BufCap = params.DictCap
	r = new(Reader)
	if err = InitDecoder(&r.d, lzma, params); err != nil {
		return nil, err
	}
	return r, nil
}

// Read reads data out of the LZMA reader.
func (r *Reader) Read(p []byte) (n int, err error) {
	return r.d.Read(p)
}

*/
