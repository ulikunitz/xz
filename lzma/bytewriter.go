package lzma

import (
	"errors"
	"io"
)

// bwriter implements a ByteWriter on top of a Writer
type bwriter struct {
	w   io.Writer
	buf []byte
}

// WriteByte writes a byte using the standard writer. Note that the
// function is not thread-safe.
func (bw *bwriter) WriteByte(c byte) error {
	bw.buf[0] = c
	_, err := bw.w.Write(bw.buf)
	return err
}

// ByteWriterFromWriter converts a Writer to a ByteWriter. If the Writer
// doesn't support the ByteWriter interface directly a new structure
// will be allocated and a non-thread-safe ByteWriter will be returned.
func ByteWriterFromWriter(w io.Writer) io.ByteWriter {
	if bw, ok := w.(io.ByteWriter); ok {
		return bw
	}
	return &bwriter{w, make([]byte, 1)}
}

// ErrLimit indicates that the limit has been reached.
var ErrLimit = errors.New("limit reached")

// LimitedByteWriter provides a byte writer that can be written until a
// limit is reached. The field N provides the number of remaining
// bytes.
type LimitedByteWriter struct {
	BW io.ByteWriter
	N  int64
}

// WriteByte writes a single byte to the limited byte writer. It returns
// ErrLimit if the limit has been reached. If the byte is successfully
// written the field N of the LimitedByteWriter will be decremented by
// one.
func (l *LimitedByteWriter) WriteByte(c byte) error {
	if l.N <= 0 {
		return ErrLimit
	}
	if err := l.BW.WriteByte(c); err != nil {
		return err
	}
	l.N--
	return nil
}
