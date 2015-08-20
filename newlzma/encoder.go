package newlzma

import (
	"errors"
	"io"
)

type Flags int

const (
	EOSMark Flags = 1 << iota
	Uncompressed
	NoUncompressedSize
	NoCompressedSize
	ResetState
	ResetProperties = ResetState | 1<<iota
	ResetDict       = ResetProperties | 1<<iota
)

type CodecParams struct {
	DictCap          int
	BufCap           int
	CompressedSize   int64
	UncompressedSize int64
	LC               int
	LP               int
	PB               int
	Flags            Flags
}

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
