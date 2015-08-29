package lzma2

import (
	"bytes"
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

type Parameters struct {
	LC       int
	LP       int
	PB       int
	DictSize int64
}

type segmentWriter struct {
	pw  io.Writer
	buf bytes.Buffer
	lw  *lzma.Writer
}

func newSegmentWriter(pw io.Writer, p Parameters) (w *segmentWriter, err error) {
	if pw == nil {
		return nil, errors.New("newSegmentWriter: pw argument is nil")
	}
	lp := lzma.Parameters{
		LC:           p.LC,
		LP:           p.LP,
		PB:           p.PB,
		DictSize:     p.DictSize,
		SizeInHeader: true,
	}
	lw, err := lzma.NewStreamWriter(pw, lp)
	if err != nil {
		return
	}
	w = &segmentWriter{
		pw: pw,
		lw: lw,
	}
	return
}

// number of bytes that may be written by closing the stream writer. May
// be wrong check for it.
const margin = 5 + 11

func (w *segmentWriter) Write(p []byte) (n int, err error) {
	panic("TODO")
}

func (w *segmentWriter) Flush(p []byte) (n int, err error) {
	panic("TODO")
}

func (w *segmentWriter) Close(p []byte) (n int, err error) {
	panic("TODO")
}
