package lzma2

import (
	"bytes"
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

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

// chunkWriter provides a type that takes care of writing out a single
// chunk. Since it cannot be known upfront what size the compressed
// segment will be, the compressed data has to be buffered, before it
// can be written out.
type chunkWriter struct {
	w     io.Writer
	ctype chunkType

	initialState *lzma.State
	encoder      *lzma.Encoder

	lbw lzma.LimitedByteWriter
	buf bytes.Buffer
}

func newChunkWriter(w io.Writer, state *lzma.State, dict *lzma.EncoderDict) (cw *chunkWriter, err error) {
	cw = &chunkWriter{
		w:            w,
		ctype:        cLRND,
		initialState: lzma.NewStateClone(state),
		lbw: lzma.LimitedByteWriter{
			BW: &cw.buf, N: maxCompressed},
	}
	cw.buf.Grow(maxCompressed)

	cw.encoder, err = lzma.NewEncoder(&cw.lbw, state, dict, 0)
	if err != nil {
		return nil, err
	}

	return cw, nil
}

func (cw *chunkWriter) Reopen(ctype chunkType) error {
	cw.ctype = ctype
	cw.lbw.N = maxCompressed
	cw.buf.Reset()
	cw.initialState = lzma.NewStateClone(cw.encoder.State)
	if err := cw.encoder.Reopen(&cw.lbw); err != nil {
		return err
	}
	return nil
}

func (cw *chunkWriter) Write(p []byte) (n int, err error) {
	panic("TODO")
}

func (cw *chunkWriter) writeUncompressedChunk() error {
	u := cw.encoder.Compressed()
	if u <= 0 {
		return errors.New("chunkWriter: can't write empty chunk")
	}
	switch cw.ctype {
	case cLRND:
		cw.ctype = cUD
	default:
		cw.ctype = cU
	}
	cw.encoder.State = cw.initialState

	header := chunkHeader{
		ctype:        cw.ctype,
		uncompressed: uint32(u - 1),
	}
	hdata, err := header.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err = cw.w.Write(hdata); err != nil {
		return err
	}
	_, err = cw.encoder.Dict.CopyN(cw.w, int(u))
	return err
}

func (cw *chunkWriter) writeCompressedChunk() error {
	if cw.ctype == cU || cw.ctype == cUD {
		return errors.New(
			"writeCompressedChunk: uncompressed chunktype")
	}

	u := cw.encoder.Compressed()
	if u <= 0 {
		return errors.New("writeCompressedChunk: empty chunk")
	}
	c := cw.buf.Len()
	if c <= 0 {
		return errors.New("writeCompressedChunk: no compressed data")
	}
	header := chunkHeader{
		ctype:        cw.ctype,
		uncompressed: uint32(u - 1),
		compressed:   uint16(c - 1),
		props:        cw.encoder.State.Properties,
	}
	hdata, err := header.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err = cw.w.Write(hdata); err != nil {
		return err
	}

	_, err = io.Copy(cw.w, &cw.buf)
	return err
}

func (cw *chunkWriter) Close() error {
	if err := cw.encoder.Close(); err != nil {
		return err
	}
	u := int(uncompressedHeaderLen + cw.encoder.Compressed())
	c := headerLen(cw.ctype) + cw.buf.Len()
	if u < c {
		return cw.writeUncompressedChunk()
	}
	return cw.writeCompressedChunk()
}
