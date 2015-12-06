package lzma2

import (
	"bytes"
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// Parameters describe the parameters for an LZMA2 writer.
type Parameters struct {
	Properties lzma.Properties
	DictCap    int
	BufSize    int
}

// Default defines the default parameters for the LZMA writer.
var Default = Parameters{
	Properties: lzma.Properties{LC: 3, LP: 0, PB: 2},
	DictCap:    8 * 1024 * 1024,
	BufSize:    4096,
}

// Writer supports the creation of an LZMA2 stream. But note that
// written data is buffered, so call Flush or Close to write data to the
// underlying writer. The output of two writers can be concatenated as
// long as the first writer has been flushed but not closed.
type Writer struct {
	w io.Writer

	start   *lzma.State
	encoder *lzma.Encoder

	cstate chunkState
	ctype  chunkType

	buf bytes.Buffer
	lbw lzma.LimitedByteWriter
}

// NewWriter creates an LZMA2 chunk sequence writer with the default
// parameters.
func NewWriter(lzma2 io.Writer) (w *Writer, err error) {
	return NewWriterParams(lzma2, Default)
}

// verifyProps checks the properties including the LZMA2 specific test
// that the sum of lc and lp doesn't exceed 4.
func verifyProps(p lzma.Properties) error {
	if err := p.Verify(); err != nil {
		return err
	}
	if p.LC+p.LP > 4 {
		return errors.New("lzma2: sum of lc and lp exceeds 4")
	}
	return nil
}

// NewWriterParams creates an LZMA2 chunk stream writer with the given
// parameters.
func NewWriterParams(lzma2 io.Writer, params Parameters) (w *Writer,
	err error) {

	if lzma2 == nil {
		return nil, errors.New("lzma2: writer must not be nil")
	}

	props := params.Properties
	if err = verifyProps(props); err != nil {
		return nil, err
	}

	w = &Writer{
		w:      lzma2,
		cstate: start,
		ctype:  start.defaultChunkType(),
		start:  lzma.NewState(props),
	}
	w.buf.Grow(maxCompressed)
	w.lbw = lzma.LimitedByteWriter{BW: &w.buf, N: maxCompressed}
	d, err := lzma.NewEncoderDict(params.DictCap, params.BufSize)
	if err != nil {
		return nil, err
	}
	w.encoder, err = lzma.NewEncoder(&w.lbw, lzma.NewStateClone(w.start),
		d, 0)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// written returns the number of bytes written to the current chunk
func (w *Writer) written() int {
	return int(w.encoder.Compressed()) + w.encoder.Dict.Buffered()
}

// errClosed indicates that the writer is closed.
var errClosed = errors.New("lzma2: writer closed")

// Writes data to LZMA2 stream. Note that written data will be buffered.
// Use Flush or Close to ensure that data is written to the underlying
// writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.cstate == stop {
		return 0, errClosed
	}
	for n < len(p) {
		m := maxUncompressed - w.written()
		if m <= 0 {
			panic("lzma2: maxUncompressed reached")
		}
		var q []byte
		if n+m < len(p) {
			q = p[n : n+m]
		} else {
			q = p[n:]
		}
		k, err := w.encoder.Write(q)
		n += k
		if err != nil && err != lzma.ErrLimit {
			return n, err
		}
		if err == lzma.ErrLimit || k == m {
			if err = w.flushChunk(); err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

// writeUncompressedChunk writes an uncomressed chunk to the LZMA2
// stream.
func (w *Writer) writeUncompressedChunk() error {
	u := w.encoder.Compressed()
	if u <= 0 {
		return errors.New("lzma2: can't write empty uncompressed chunk")
	}
	if u > maxUncompressed {
		panic("overrun of uncompressed data limit")
	}
	switch w.ctype {
	case cLRND:
		w.ctype = cUD
	default:
		w.ctype = cU
	}
	w.encoder.State = w.start

	header := chunkHeader{
		ctype:        w.ctype,
		uncompressed: uint32(u - 1),
	}
	hdata, err := header.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err = w.w.Write(hdata); err != nil {
		return err
	}
	_, err = w.encoder.Dict.CopyN(w.w, int(u))
	return err
}

// writeCompressedChunk writes a compressed chunk to the underlying
// writer.
func (w *Writer) writeCompressedChunk() error {
	if w.ctype == cU || w.ctype == cUD {
		panic("chunk type uncompressed")
	}

	u := w.encoder.Compressed()
	if u <= 0 {
		return errors.New("writeCompressedChunk: empty chunk")
	}
	if u > maxUncompressed {
		panic("overrun of uncompressed data limit")
	}
	c := w.buf.Len()
	if c <= 0 {
		panic("no compressed data")
	}
	if c > maxCompressed {
		panic("overrun of compressed data limit")
	}
	header := chunkHeader{
		ctype:        w.ctype,
		uncompressed: uint32(u - 1),
		compressed:   uint16(c - 1),
		props:        w.encoder.State.Properties,
	}
	hdata, err := header.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err = w.w.Write(hdata); err != nil {
		return err
	}
	_, err = io.Copy(w.w, &w.buf)
	return err
}

// writes a single chunk to the underlying writer.
func (w *Writer) writeChunk() error {
	u := int(uncompressedHeaderLen + w.encoder.Compressed())
	c := headerLen(w.ctype) + w.buf.Len()
	if u < c {
		return w.writeUncompressedChunk()
	}
	return w.writeCompressedChunk()
}

// flushChunk terminates the current chunk. The encoder will be reset
// to support the next chunk.
func (w *Writer) flushChunk() error {
	if w.written() == 0 {
		return nil
	}
	var err error
	if err = w.encoder.Close(); err != nil {
		return err
	}
	if err = w.writeChunk(); err != nil {
		return err
	}
	w.buf.Reset()
	w.lbw.N = maxCompressed
	if err = w.encoder.Reopen(&w.lbw); err != nil {
		return err
	}
	if err = w.cstate.next(w.ctype); err != nil {
		return err
	}
	w.ctype = w.cstate.defaultChunkType()
	w.start = lzma.NewStateClone(w.encoder.State)
	return nil
}

// Flush writes all buffered data out to the underlying stream. This
// could result in multiple chunks to be created.
func (w *Writer) Flush() error {
	if w.cstate == stop {
		return errClosed
	}
	for w.written() > 0 {
		if err := w.flushChunk(); err != nil {
			return err
		}
	}
	return nil
}

// Close terminates the LZMA2 stream with an EOS chunk.
func (w *Writer) Close() error {
	if w.cstate == stop {
		return errClosed
	}
	if err := w.Flush(); err != nil {
		return nil
	}
	// write zero byte
	n, err := w.w.Write([]byte{0})
	if err != nil {
		return nil
	}
	if n == 0 {
		return errors.New(
			"lzma2: end-of-stream chunk hasn't been written")
	}
	w.cstate = stop
	return nil
}
