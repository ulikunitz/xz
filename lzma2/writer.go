package lzma2

import "io"

type Writer struct {
	// TODO
	params Parameters
}

func NewWriter(w io.Writer) (lw *Writer, err error) {
	panic("TODO")
}

func NewWriterP(w io.Writer, p Parameters) (lw *Writer, err error) {
	panic("TODO")
}

func (lw *Writer) Write(p []byte) (n int, err error) {
	panic("TODO")
}

func (lw *Writer) Flush() error {
	panic("TODO")
}

func (lw *Writer) Close() error {
	panic("TODO")
}

func (lw *Writer) Parameters() Parameters {
	panic("TODO")
}

func (lw *Writer) SetParameters(p Parameters) {
	panic("TODO")
}
