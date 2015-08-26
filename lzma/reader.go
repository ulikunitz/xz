package lzma

import "io"

type Reader struct {
	d Decoder
}

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

func (r *Reader) Read(p []byte) (n int, err error) {
	return r.d.Read(p)
}
