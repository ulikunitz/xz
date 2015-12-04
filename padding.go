package xz

import (
	"errors"
	"io"
)

// padWriter supports the writing of padding aligned to a block size.
// padWriter remembers errors.
type padWriter struct {
	w         io.Writer
	n         int64
	blockSize int
	pad       []byte
}

// newPadWriter creates a new pad writer.
func newPadWriter(w io.Writer, blockSize int) *padWriter {
	if blockSize < 1 {
		blockSize = 1
	}
	return &padWriter{
		w:         w,
		blockSize: blockSize,
		pad:       make([]byte, blockSize-1),
	}
}

// Write writes the slide to the underlying writer.
func (w *padWriter) Write(p []byte) (n int, err error) {
	n, err = w.w.Write(p)
	w.n += int64(n)
	return n, err
}

// Pad writes the pad to align the stream with the block size.
func (w *padWriter) Pad() (n int, err error) {
	k := int(w.n % int64(w.blockSize))
	if k == 0 {
		return 0, nil
	}
	n, err = w.Write(w.pad[:w.blockSize-k])
	return n, err
}

type bReader struct {
	io.Reader
	p []byte
}

// byteReader converts a reader to a byte reader. If the reader doesn't
// support the ByteReader interface, it returns a wrapper.
func byteReader(r io.Reader) io.ByteReader {
	if br, ok := r.(io.ByteReader); ok {
		return br
	}
	return &bReader{r, make([]byte, 1)}
}

// ReadByte reads a single byte.
func (br *bReader) ReadByte() (c byte, err error) {
	n, err := br.Read(br.p)
	if n == 0 {
		if err != nil {
			return 0, err
		}
		return 0, errors.New("bReader: no data")
	}
	return br.p[0], nil
}

// PadReader supports the reading of pads from a reader.
type padReader struct {
	r         io.Reader
	br        io.ByteReader
	n         int64
	blockSize int
	p         []byte
}

// newPadReader creates a new pad reader.
func newPadReader(r io.Reader, blockSize int) *padReader {
	if blockSize < 1 {
		blockSize = 1
	}
	return &padReader{
		r:         r,
		br:        byteReader(r),
		blockSize: blockSize,
		p:         make([]byte, blockSize-1),
	}
}

// Read reads data from the underlying reader.
func (r *padReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	r.n += int64(n)
	return n, err
}

// ReadByte reads a byte from the pad reader.
func (r *padReader) ReadByte() (c byte, err error) {
	c, err = r.br.ReadByte()
	if err == nil {
		r.n += 1
	}
	return c, err
}

// Pad reads the padding from the underlying reader.
func (r *padReader) Pad() (n int, err error) {
	k := int(r.n % int64(r.blockSize))
	if k == 0 {
		return 0, nil
	}
	if n, err = io.ReadFull(r, r.p[:r.blockSize-k]); err != nil {
		return n, err
	}
	for _, c := range r.p[:n] {
		if c != 0 {
			return n, errors.New("xz: non-zero padding byte")
		}
	}
	return n, nil
}
