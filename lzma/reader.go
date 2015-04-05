package lzma

import (
	"io"

	"github.com/uli-go/xz/lzbase"
)

// Reader supports the decoding of data in the classic LZMA format.
type Reader struct {
	lzbase.Reader
}

// NewReader creates a new LZMA reader.
func NewReader(r io.Reader) (lr *Reader, err error) {
	panic("TODO")
}
