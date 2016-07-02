package lzma

import (
	"errors"
	"io"
)

// ReaderConfig stores the parameters for the reader of the classic LZMA
// format.
type ReaderConfig struct {
	DictCap int
}

// fill converts the zero values of the config to the default values.
func (c *ReaderConfig) fill() {
	if c.DictCap == 0 {
		c.DictCap = 8 * 1024 * 1024
	}
}

// verify checks the reader configuration for errors.
func (c *ReaderConfig) verify() error {
	if !(MinDictCap <= c.DictCap && c.DictCap <= MaxDictCap) {
		return errors.New("lzma: dictionary capacity is out of range")
	}
	return nil
}

// Reader provides a reader for LZMA files or streams.
type Reader struct {
	lzma io.Reader
	h    Header
	d    *decoder
}

// NewReader creates a new reader for an LZMA stream using the classic
// format. NewReader reads and checks the header of the LZMA stream.
func NewReader(lzma io.Reader) (r *Reader, err error) {
	return ReaderConfig{}.NewReader(lzma)
}

// NewReader creates a new reader for an LZMA stream in the classic
// format. The function reads and verifies the the header of the LZMA
// stream.
func (c ReaderConfig) NewReader(lzma io.Reader) (r *Reader, err error) {
	c.fill()
	if err = c.verify(); err != nil {
		return nil, err
	}
	data := make([]byte, HeaderLen)
	if _, err := io.ReadFull(lzma, data); err != nil {
		if err == io.EOF {
			return nil, errors.New("lzma: unexpected EOF")
		}
		return nil, err
	}
	r = &Reader{lzma: lzma}
	if err = r.h.unmarshalBinary(data); err != nil {
		return nil, err
	}
	if r.h.DictCap < MinDictCap {
		return nil, errors.New("lzma: dictionary capacity too small")
	}
	dictCap := r.h.DictCap
	if c.DictCap > dictCap {
		dictCap = c.DictCap
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
func (r *Reader) EOSMarker() bool {
	return r.d.eosMarker
}

// Read returns uncompressed data.
func (r *Reader) Read(p []byte) (n int, err error) {
	return r.d.Read(p)
}
