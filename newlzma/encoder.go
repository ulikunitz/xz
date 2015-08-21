package newlzma

import (
	"errors"
	"io"
)

type Encoder struct {
}

func NewEncoder(w io.Writer, p CodecParams) (e *Encoder, err error) {
	panic("TODO(uk)")
}

var ErrResetRequired = errors.New("encoder reset is required")

func (e *Encoder) Write(p []byte) (n int, err error) {
	panic("TODO(uk)")
}

func (e *Encoder) Flush() error {
	panic("TODO(uk)")
}

func (e *Encoder) Compressed() int64 {
	panic("TODO(uk)")
}

func (e *Encoder) Uncompressed() int64 {
	panic("TODO(uk)")
}

func (e *Encoder) Reset(w io.Writer, p CodecParams) error {
	panic("TODO(uk)")
}
