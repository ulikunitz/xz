package lzma

import "io"

type XWriter struct {
}

func NewXWriter(lzma io.Writer) *XWriter {
	w, err := NewXWriterParams(lzma, WriterParams{})
	if err != nil {
		panic(err)
	}
	return w
}

func NewXWriterParams(lzma io.Writer, p WriterParams) (w *XWriter, err error) {
	panic("TODO")
}

func (w *XWriter) Write(p []byte) (n int, err error) {
	panic("TODO")
}

func (w *XWriter) Close() error {
	panic("TODO")
}
