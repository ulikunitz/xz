package lzma

import (
	"bufio"
	"io"
)

// MinDictCap and MaxDictCap provide the range of supported dictionary
// capacities.
const (
	MinDictCap = 1 << 12
	MaxDictCap = 1<<32 - 1
)

// Writer compresses data in the classic LZMA format.
type Writer struct {
	Parameters Parameters
	bw         *bufio.Writer
	e          Encoder
}

// NewWriter creates a new writer for the classic LZMA format.
func NewWriter(lzma io.Writer) (w *Writer, err error) {
	return NewWriterParams(lzma, &Default)
}

// NewWriterParams supports parameters for the creation of an LZMA
// writer.
func NewWriterParams(lzma io.Writer, p *Parameters) (w *Writer, err error) {
	p.normalizeWriter()
	if err = p.verifyWriter(); err != nil {
		return nil, err
	}

	w = &Writer{
		Parameters: *p,
	}

	bw, ok := lzma.(io.ByteWriter)
	if !ok {
		w.bw = bufio.NewWriter(lzma)
		bw = w.bw
		if err := writeHeader(w.bw, p); err != nil {
			return nil, err
		}
	} else {
		if err := writeHeader(lzma, p); err != nil {
			return nil, err
		}
	}

	props, err := NewProperties(p.LC, p.LP, p.PB)
	if err != nil {
		return nil, err
	}
	state := NewState(props)

	dict, err := NewEncoderDict(p.DictCap, p.BufCap)
	if err != nil {
		return nil, err
	}

	codecParams := CodecParams{
		Size:      p.Size,
		EOSMarker: p.EOSMarker,
	}
	if err = w.e.Init(bw, state, dict, codecParams); err != nil {
		return nil, err
	}
	return w, nil
}

// Write puts data into the Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.Parameters.Size >= 0 {
		m := w.Parameters.Size
		m -= w.e.Uncompressed() + int64(w.e.Dict.Buffered())
		if m < 0 {
			m = 0
		}
		if m < int64(len(p)) {
			p = p[:m]
			err = ErrNoSpace
		}
	}
	var werr error
	if n, werr = w.e.Write(p); werr != nil {
		err = werr
	}
	return n, err
}

// Close closes the writer stream. It ensures that all data from the
// buffer will be compressed and the LZMA stream will be finished.
func (w *Writer) Close() error {
	if w.Parameters.Size >= 0 {
		n := w.e.Uncompressed() + int64(w.e.Dict.Buffered())
		if n != w.Parameters.Size {
			return errSize
		}
	}
	err := w.e.Wash()
	if err == nil {
		err = w.e.Close()
	}

	if w.bw != nil {
		ferr := w.bw.Flush()
		if err == nil {
			err = ferr
		}
	}
	return err
}
