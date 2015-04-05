package lzma

import (
	"io"

	"github.com/uli-go/xz/lzbase"
)

// Reader supports the decoding of data in the classic LZMA format.
type Reader struct {
	lzbase.Reader
	params *Parameters
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
	dict, err := lzbase.NewReaderDict(int64(p.DictSize), p.BufferSize)
	if err != nil {
		return nil, err
	}
	oc := lzbase.NewOpCodec(p.Properties(), dict)
	lr = &Reader{params: p}
	err = lzbase.InitReader(&lr.Reader, r, oc,
		lzbase.Parameters{Size: p.Size, SizeInHeader: p.SizeInHeader})
	if err != nil {
		return nil, err
	}
	return lr, err
}

// Parameters returns the parameters of the LZMA reader. The parameters reflect
// the status provided by the header of the LZMA file.
func (lr *Reader) Parameters() Parameters {
	return *lr.params
}
