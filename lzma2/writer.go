package lzma2

import "io"

// Parameters describe the parameters for an LZMA2 writer.
type Parameters struct {
	LC      int
	LP      int
	PB      int
	DictCap int
	BufSize int
}

// Writer writes a sequence of LZMA2 chunks. The first chunk will always
// reset the dictionary. The Writer will not terminate the chunk
// sequence with an end-of-stream chunk. Use WriteEOS for writing the
// end of stream chunk.
type Writer struct {
	// TODO
}

// NewWriter creates an LZMA2 chunk sequence writer with the default
// parameters.
func NewWriter(lzma2 io.Writer) (w *Writer, err error) {
	panic("TODO")
}

// NewWriterParams creates an LZMA2 chunk stream writer with the given
// parameters.
func NewWriterParams(lzma2 io.Writer, params Parameters) (w *Writer,
	err error) {

	panic("TODO")
}

// Writes data to the LZMA2 chunk stream.
func (w *Writer) Write(p []byte) (n int, err error) {
	panic("TODO")
}

// Flush terminates the current chunk. If data will be provided later a
// new chunk will be created.
func (w *Writer) Flush() error {
	panic("TODO")
}

// Close terminates the chunk sequence. It doesn't write an
// end-of-stream chunk. Use WriteEOS to write such a chunk.
func (w *Writer) Close() error {
	panic("TODO")
}

// The FileWriter writes LZMA2 files with a leading dictionary capacity
// byte and an end-of-stream chunk.
type FileWriter struct {
	Writer
}

// NewFileWriter creates an LZMA2 file writer. It writes the initial
// dictionary capacity byte.
func NewFileWriter(lzma2 io.Writer) (w *FileWriter, err error) {
	panic("TODO")
}

// Close closes the file writer by terminating the active chunk and
// writing an end-of-stream chunk.
func (w *FileWriter) Close() error {
	panic("TODO")
}
