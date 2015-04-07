package lzma2

import "io"

type Reader struct {
	// TODO
	params Parameters
}

func NewReader(r io.Reader) (lr *Reader, err error) {
	panic("TODO")
}

func NewReaderDictSize(r io.Reader, dictSize DictSize) (lr *Reader, err error) {
	panic("TODO")
}

func (lr *Reader) Read(p []byte) (n int, err error) {
	panic("TODO")
}
