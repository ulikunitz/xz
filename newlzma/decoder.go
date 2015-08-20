package newlzma

import (
	"errors"
	"io"
)

type Decoder struct {
}

func NewDecoder(r io.Reader, p CodecParams) (e *Decoder, err error) {
	panic("TODO(uk)")
}

var (
	EOS                      = errors.New("end of stream")
	ErrMissingEOSMarker      = errors.New("EOS marker is missing")
	ErrUnexpectedEOF         = errors.New("EOF unexpected")
	ErrWrongCompressedSize   = errors.New("given compressed size wrong")
	ErrWrongUncompressedSize = errors.New("given uncompressed size wrong")
)

func (d *Decoder) Read(p []byte) (n int, err error) {
	panic("TODO(uk)")
}

func (d *Decoder) Compressed() int64 {
	panic("TODO(uk)")
}

func (d *Decoder) Uncompressed() int64 {
	panic("TODO(uk)")
}

func (d *Decoder) Reset(r io.Reader, p CodecParams) error {
	panic("TODO(uk)")
}
