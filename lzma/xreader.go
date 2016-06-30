package lzma

import (
	"errors"
	"io"
)

// XReader provides a reader for LZMA files or streams.
type XReader struct {
	lzma io.Reader
	h    Header
	d    *decoder
}

// NewXReader creates a new reader for an LZMA stream using the classic
// format. NewReader reads and checks the header of the LZMA stream.
func NewXReader(lzma io.Reader) (r *XReader, err error) {
	return NewXReaderParams(lzma, nil)
}

// NewXReaderParams creates a new reader for an LZMA stream using the classic
// format. It will use the provided parameters. The function reads and
// checks the header of the LZMA stream.
func NewXReaderParams(lzma io.Reader, params *ReaderParams) (r *XReader, err error) {
	params = fillReaderParams(params)
	if err = params.Verify(); err != nil {
		return nil, err
	}
	data := make([]byte, HeaderLen)
	if _, err := io.ReadFull(lzma, data); err != nil {
		if err == io.EOF {
			return nil, errors.New("lzma: unexpected EOF")
		}
		return nil, err
	}
	r = &XReader{lzma: lzma}
	if err = r.h.unmarshalBinary(data); err != nil {
		return nil, err
	}
	if r.h.DictCap < MinDictCap {
		return nil, errors.New("lzma: dictionary capacity too small")
	}
	dictCap := r.h.DictCap
	if params.DictCap > dictCap {
		dictCap = params.DictCap
	}

	state := newState(r.h.Properties)
	dict, err := newDecoderDict(dictCap)
	if err != nil {
		return nil, err
	}
	r.d, err = newDecoder(ByteReader(lzma), state, dict, r.h.Size)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// EOSMarker indicates that an EOS marker has been encountered.
func (r *XReader) EOSMarker() bool {
	return r.d.eosMarker
}

// Read returns uncompressed data.
func (r *XReader) Read(p []byte) (n int, err error) {
	return r.d.Read(p)
}
