package lzma

import "io"

// Writer supports the LZMA compression of a file. It cannot support flushing
// because of the arithmethic coder.
type Writer struct {
	unpackLen  uint64
	writtenLen uint64
}

// NewWriter creates a new LZMA writer using the given properties. It doesn't
// provide an unpack length and creates an explicit end of stream. The classic
// LZMA header will be created.
func NewWriter(w io.Writer, p *Properties) (*Writer, error) {
	return NewWriterLenEOS(w, p, NoUnpackLen, true)
}

// NewWriterLen creates a new LZMA writer and a predefined length. There will
// be no end-of-stream marker created unless NoUnpackLen is used as length.
func NewWriterLen(w io.Writer, p *Properties, length uint64) (*Writer, error) {
	return NewWriterLenEOS(w, p, length, false)
}

// NewWriterLenEOS creates a new LZMA writer. A predefinied length can be
// provided and the writing of an end-of-stream marker can be controlled. If
// the argument NoUnpackLen will be provided for the lenght a end-of-stream
// marker will be written regardless of the eos parameter.
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
