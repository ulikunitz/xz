package lzma2

import (
	"errors"
	"io"
)

// The EOS error indicates that an LZMA2 end-of-stream chunk has been
// encountered.
var EOS = errors.New("lzma2 end-of-stream")

// A reader supports the reading of LZMA2 chunk sequences. Note that the
// first chunk should have a dictionary reset and the first compressed
// chunk a properties reset. The chunk sequence may not be terminated by
// an end-of-stream chunk.
type Reader struct {
	// TODO
}

// NewReader creates a reader for an LZMA2 chunk sequence with the given
// dictionary capacity.
func NewReader(lzma2 io.Reader, dictCap int) (r *Reader, err error) {
	panic("TODO")
}

// Read reads data from the LZMA2 chunk sequence. If an end-of-stream
// chunk is encountered EOS is returned, it the sequence stops without
// an end-of-stream chunk io.EOF is returned.
func (r *Reader) Read(p []byte) (n int, err error) {
	panic("TODO")
}

// The file reader supports the reading of LZMA2 files, where the first
// chunk is preceded by the dictionary size.
type FileReader struct {
	r Reader
}

// NewFileReader creates a reader for LZMA2 files, where the dictionary
// capacity is encoded in the first byte.
func NewFileReader(lzma2File io.Reader) (r *FileReader, err error) {
	panic("TODO")
}

// Reads data from the file reader. It returns io.EOF if the end of the
// file is encountered.
func (r *FileReader) Read(p []byte) (n int, err error) {
	panic("TODO")
}
