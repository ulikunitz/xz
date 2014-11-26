package lzma

import "io"

// Writer supports the LZMA compression of a file. Note that the LZMA encoder
// doesn't support flushing because of the arithmetic coding included.
type Writer struct {
	unpackLen  uint64
	writtenLen uint64
}

func NewWriter(w io.Writer, p *Properties) (*Writer, error) {
	return NewWriterLenEOS(w, p, NoUnpackLen, true)
}

func NewWriterLen(w io.Writer, p *Properties, length uint64) (*Writer, error) {
	return NewWriterLenEOS(w, p, length, false)
}

func NewWriterLenEOS(w io.Writer, p *Properties, length uint64, eos bool) (*Writer, error) {
	if length == NoUnpackLen {
		eos = true
	}
	panic("TODO")
}

// Writes data into the writer buffer.
func (l *Writer) Write(p []byte) (int, error) {
	panic("TODO")
}

// Close flushes all data out and writes the EOS marker if requested.
func (l *Writer) Close() error {
	panic("TODO")
}

/*

# Ideas

We need an encoder dictionary in which data can be written. For the encoder the
dictionary should be not be bigger than the history size. We continue to use a
ring buffer.

*/
