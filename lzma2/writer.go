package lzma2

import (
	"bytes"
	"errors"
	"fmt"
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

// Default defines the default parameters for the LZMA writer.
var Default = Parameters{
	LC:      3,
	LP:      0,
	PB:      2,
	DictCap: 8 * 1024 * 1024,
	BufSize: 4096,
}

// properties returns the Properties value for the Parameters.
func (p *Parameters) properties() (props lzma.Properties, err error) {
	if props, err = lzma.NewProperties(p.LC, p.LP, p.PB); err != nil {
		return props, err
	}
	if p.LC+p.LP > 4 {
		return props, fmt.Errorf(
			"lzma2: sum of lc and lp must not exceed four")
	}
	return props, nil
}

// Writer writes a sequence of LZMA2 chunks. The first chunk will always
// reset the dictionary. The Writer will not terminate the chunk
// sequence with an end-of-stream chunk. Use WriteEOS for writing the
// end of stream chunk.
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

// NewWriterParams creates an LZMA2 chunk stream writer with the given
// parameters.
func NewWriterParams(lzma2 io.Writer, params Parameters) (w *Writer,
	err error) {

	if lzma2 == nil {
		return nil, errors.New("lzma2: writer must not be nil")
	}
	props, err := params.properties()
	if err != nil {
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

func (w *Writer) compress(flags lzma.CompressFlags) error {
	var err error
	for {
		r := maxUncompressed - w.encoder.Compressed()
		if r <= 0 {
			if r < 0 {
				panic("maxUncompressed limit overrun")
			}
			if err = w.Flush(); err != nil {
				return err
			}
			continue
		}

		n, f := int(r), flags
		if b := w.encoder.Dict.Buffered(); b < n {
			n = b
			if n == 0 {
				return nil
			}
		} else {
			f |= lzma.All
		}
		if n, err = w.encoder.Compress(n, f); err != nil {
			if err == lzma.ErrLimit {
				if err = w.Flush(); err == nil {
					continue
				}
			}
			return err
		}
		if n == 0 {
			return nil
		}
	}
}

// Writes data to the LZMA2 chunk stream.
func (w *Writer) Write(p []byte) (n int, err error) {
	dict := w.encoder.Dict
	for {
		k, err := dict.Write(p[n:])
		n += k
		if err != lzma.ErrNoSpace {
			return n, err
		}
		if err = w.compress(0); err != nil {
			return n, err
		}
	}
}

func (w *Writer) writeUncompressedChunk() error {
	u := w.encoder.Compressed()
	if u <= 0 {
		return errors.New("lzma2: can't write empty chunk")
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

func (w *Writer) writeChunk() error {
	u := int(uncompressedHeaderLen + w.encoder.Compressed())
	c := headerLen(w.ctype) + w.buf.Len()
	if u < c {
		return w.writeUncompressedChunk()
	}
	return w.writeCompressedChunk()
}

// Flush terminates the current chunk. If data will be provided later a
// new chunk will be created.
func (w *Writer) Flush() error {
	var err error
	if w.encoder.Compressed() == 0 {
		return nil
	}
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

// Close terminates the chunk sequence. It doesn't write an
// end-of-stream chunk. Use WriteEOS to write such a chunk.
func (w *Writer) Close() error {
	var err error
	if err = w.compress(lzma.All); err != nil {
		return err
	}
	if w.encoder.Compressed() == 0 {
		return nil
	}
	if err = w.encoder.Close(); err != nil {
		return err
	}
	if err = w.writeChunk(); err != nil {
		return err
	}
	w.cstate = stop
	return nil
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
