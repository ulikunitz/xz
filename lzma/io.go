package lzma

import (
	"errors"
	"fmt"
	"io"
)

// The file contains simple types providing functionality on top of the
// Wr(iter and Reader interfaces provided by the io package.

// bReader implements a ByteReader on top of a Reader.
type bReader struct {
	io.Reader
}

// errNoByte indicates that ReadByte cannot return a byte.
var errNoByte = errors.New("ByteReader: no byte available")

// ReadByte reads a single byte from the underlying reader. If no byte
// is read returns an error.
func (br *bReader) ReadByte() (c byte, err error) {
	a := make([]byte, 1)
	if n, err := br.Read(a); n == 0 {
		if err == nil {
			err = errNoByte
		}
		return 0, err
	}
	return a[0], nil
}

// newByteReader converts an io.Reader to a byteReader.
func newByteReader(r io.Reader) io.ByteReader {
	if br, ok := r.(io.ByteReader); ok {
		return br
	}
	// error after the return of 1 byte has to be ignored here
	return &bReader{r}
}

// bWriter implements a ByteWriter on top of a Writer.
type bWriter struct {
	io.Writer
}

// WriteByte writes a single byte. If the byte couldn't be written an
// error is returned.
func (bw *bWriter) WriteByte(c byte) error {
	n, err := bw.Write([]byte{c})
	if n == 0 {
		if err == nil {
			panic("WriteByte: Write returned 0 without error")
		}
		return err
	}
	if err != nil {
		panic(fmt.Errorf("WriteByte: " +
			"Write returned 1 but returned an error"))
	}
	return nil
}

// newByteWriter converts a Writer into a ByteWriter.
func newByteWriter(w io.Writer) io.ByteWriter {
	if bw, ok := w.(io.ByteWriter); ok {
		return bw
	}
	return &bWriter{w}
}

// errLimit is returned if a given limit has been reached.
var errLimit = errors.New("limit reached")

// lbcReader provides a ByteReader that counts the bytes read and has a
// limit. Using MaxInt64 as the limit results in a practically unlimited
// reader.
type lbcReader struct {
	br    io.ByteReader
	n     int64
	limit int64
}

// ReadByte reads a single byte. The function will return ErrLimit if
// the limit has been reached.
func (r *lbcReader) ReadByte() (c byte, err error) {
	// >= required because r.limit might be negative
	if r.n >= r.limit {
		return 0, io.EOF
	}
	c, err = r.br.ReadByte()
	if err == nil {
		r.n++
	}
	return
}

// limitByteReader converts the Reader into a ByteReader that has a
// limit for the bytes read but can provide the bytes read and the
// limit.
func limitByteReader(r io.Reader, limit int64) *lbcReader {
	return &lbcReader{br: newByteReader(r), limit: limit}
}

// lbcWriter provides a ByteWriter that counts bytes written and
// supports a limit. Choose MaxInt64 as limit ot get a practically
// unlimited Writer.
type lbcWriter struct {
	bw    io.ByteWriter
	n     int64
	limit int64
}

// WriteByte writes a single byte to the underlying writer. If the limit
// is reached an error will be returned.
func (w *lbcWriter) WriteByte(c byte) error {
	if w.n >= w.limit {
		return errLimit
	}
	if err := w.bw.WriteByte(c); err != nil {
		return err
	}
	w.n++
	return nil
}

// limitByteWriter returns a ByteWriter that counts the bytes written
// and adheres to a limit.
func limitByteWriter(w io.Writer, limit int64) *lbcWriter {
	return &lbcWriter{bw: newByteWriter(w), limit: limit}
}
