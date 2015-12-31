package lzma2

import (
	"bytes"
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// WriterParams describes the parameters for the LZMA2 writer.
type WriterParams struct {
	Properties lzma.Properties
	DictCap    int
	// size of lookahead buffer
	BufSize int
}

// Verify verifies LZMA2 writer parameters for correctness.
func (p *WriterParams) Verify() error {
	var err error

	// dictionary capacity
	if err = verifyDictCap(p.DictCap); err != nil {
		return err
	}

	// properties
	if err = p.Properties.Verify(); err != nil {
		return err
	}
	if p.Properties.LC+p.Properties.LP > 4 {
		return errors.New("lzma2: sum of lc and lp exceeds 4")
	}

	// buffer size
	if p.BufSize < 1 {
		return errors.New(
			"lzma2: lookahead buffer size must be larger than zero")
	}

	return nil
}

// WriterDefaults defines the defaults for the LZMA2 writer parameters.
var WriterDefaults = WriterParams{
	Properties: lzma.Properties{LC: 3, LP: 0, PB: 2},
	DictCap:    8 * 1024 * 1024,
	BufSize:    4096,
}

// verifyDictCap verifies values for the dictionary capacity.
func verifyDictCap(dictCap int) error {
	if !(1 <= dictCap && int64(dictCap) <= lzma.MaxDictCap) {
		return errors.New("lzma2: dictionary capacity is out of range")
	}
	return nil
}

// Writer supports the creation of an LZMA2 stream. But note that
// written data is buffered, so call Flush or Close to write data to the
// underlying writer. The Close method writes the end-of-stream marker
// to the stream. So you may be able to concatenate the output of two
// writers as long the output of the first writer has only been flushed
// but not closed.
//
// Any change to the fields Properties, DictCap must be done before the
// first call to Write, Flush or Close.
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
// parameters and options.
func NewWriter(lzma2 io.Writer) *Writer {
	w, err := NewWriterParams(lzma2, &WriterDefaults)
	if err != nil {
		panic(err)
	}
	return w
}

// NewWriterParams creates a new LZMA2 chunk sequence writer using the
// given parameters. The parameters will be verified for correctness.
func NewWriterParams(lzma2 io.Writer, params *WriterParams) (w *Writer, err error) {
	if err = params.Verify(); err != nil {
		return nil, err
	}

	w = &Writer{
		w:      lzma2,
		start:  lzma.NewState(params.Properties),
		cstate: start,
		ctype:  start.defaultChunkType(),
	}
	w.buf.Grow(maxCompressed)
	w.lbw = lzma.LimitedByteWriter{BW: &w.buf, N: maxCompressed}
	d, err := lzma.NewEncoderDict(params.DictCap, params.BufSize)
	if err != nil {
		return nil, err
	}
	w.encoder, err = lzma.NewEncoder(&w.lbw, lzma.CloneState(w.start), d, 0)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// written returns the number of bytes written to the current chunk
func (w *Writer) written() int {
	if w.encoder == nil {
		return 0
	}
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
	w.start = lzma.CloneState(w.encoder.State)
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
	// write zero byte EOS chunk
	_, err := w.w.Write([]byte{0})
	if err != nil {
		return err
	}
	w.cstate = stop
	return nil
}
