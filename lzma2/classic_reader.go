package lzma2

import "io"

// ClassicReader supports the decoding of compressed information using the
// classic LZMA format. The xz file format uses the LZMA2 format.
type ClassicReader struct {
	baseReader
}

// NewClassicReader creates a reader for compressed data in the LZMA format.
func NewClassicReader(r io.Reader) (cr *ClassicReader, err error) {
	panic("TODO")
}
