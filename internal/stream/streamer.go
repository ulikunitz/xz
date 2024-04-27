// Package stream supports implements Streamers which track the offset in the
// file and allows the discarding of data.
package stream

import (
	"errors"
	"io"
	"math"
)

// Streamer is a reader that supports discarding input and keeps track of the
// number of bytes read.
type Streamer interface {
	io.Reader
	Discard64(n int64) (discarded int64, err error)
	Offset() int64
}

// simpleReader works on a pure io.Reader.
type simpleReader struct {
	r   io.Reader
	buf []byte
	off int64
}

// newSimpleReader creates a [simpleReader] from an io.Reader.
func newSimpleReader(r io.Reader) *simpleReader {
	return &simpleReader{
		r:   r,
		buf: make([]byte, 16*1024),
	}
}

// Offset returns the byte offset since the simpleReader was initialized.
func (r *simpleReader) Offset() int64 {
	return r.off
}

// Read reads data into p. It returns the number of bytes read into p. The
// offset will be updated accordingly.
func (r *simpleReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	r.off += int64(n)
	return n, err
}

// Discard64 discards n bytes from the reader using the buffer of the
// simpleReader structure. It returns an error if n < 0 or if the underlying
// Read method returns an error.
func (r *simpleReader) Discard64(n int64) (discarded int64, err error) {
	if n <= 0 {
		if n < 0 {
			return 0, errors.New("discard: negative count")
		}
		return 0, nil
	}

	p := r.buf[:cap(r.buf)]
	k := n
	for k > 0 {
		if k < int64(len(p)) {
			p = p[:k]
		}
		s, err := r.Read(p)
		k -= int64(s)
		if err != nil {
			n -= k
			r.off += n
			return n, err
		}
	}

	r.off += n
	return n, nil
}

// readSeeker implements the Reader interface for an io.ReadSeeker.
type readSeeker struct {
	io.ReadSeeker
}

// newReadSeeker creates a [readSeeker] from an io.ReadSeeker.
func newReadSeeker(r io.ReadSeeker) (d *readSeeker, err error) {
	_, err = r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	return &readSeeker{ReadSeeker: r}, nil
}

// Offset returns the current offset as returned by Seek(0, io.SeekCurrent).
func (s *readSeeker) Offset() int64 {
	off, err := s.Seek(0, io.SeekCurrent)
	if err != nil {
		panic("stream: failed to get offset by calling Seek")
	}
	return off
}

// Discard64 discards n bytes from the reader using the Seek method of the
// underlying   io.ReadSeeker. It returns an error if n < 0 or if the underlying
// Seek method returns an error.
func (s *readSeeker) Discard64(n int64) (discarded int64, err error) {
	if n <= 0 {
		if n < 0 {
			return 0, errors.New("discard: negative count")
		}
		return 0, nil
	}

	_, err = s.Seek(n, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return n, err
}

// intReader combines an io.Reader with a Discard method. The bufio.Reader
// implements this interface.
type intReader interface {
	io.Reader
	Discard(n int) (discarded int, err error)
}

// intWrapper wraps an [intReader] to implement the [Reader] interface.
type intWrapper struct {
	r   intReader
	off int64
}

// newIntWrapper creates an [intWrapper] from an [intReader].
func newIntWrapper(r intReader) *intWrapper {
	return &intWrapper{r: r}
}

// Offset returns the byte offset since the intWrapper was initialized.
func (w *intWrapper) Offset() int64 {
	return w.off
}

// Read reads data into p. It returns the number of bytes read into p. The
// offset will be updated accordingly.
func (w *intWrapper) Read(p []byte) (n int, err error) {
	n, err = w.r.Read(p)
	w.off += int64(n)
	return n, err
}

// Discard64 discards n bytes from the reader using the Discard method of the
// underlying [intReader]. It returns an error if n < 0 or if the underlying
// Discard method returns an error.
func (w *intWrapper) Discard64(n int64) (discarded int64, err error) {
	if n <= 0 {
		if n < 0 {
			return 0, errors.New("discard: negative count")
		}
		return 0, nil
	}
	for n > math.MaxInt {
		d, err := w.r.Discard(math.MaxInt)
		discarded += int64(d)
		if err != nil {
			w.off += discarded
			return discarded, err
		}
		n -= int64(d)
	}

	d, err := w.r.Discard(int(n))
	discarded += int64(d)
	w.off += int64(discarded)
	return discarded, err
}

// Wrap wraps an io.Reader to implement the [Reader] interface with the
// Discard64 method. It will use a Seek method if supported and a possible
// Discard method.
func Wrap(r io.Reader) Streamer {
	if s, ok := r.(Streamer); ok {
		return s
	}
	if rs, ok := r.(io.ReadSeeker); ok {
		s, err := newReadSeeker(rs)
		if err == nil {
			return s
		}
	}
	if ir, ok := r.(intReader); ok {
		return newIntWrapper(ir)
	}
	return newSimpleReader(r)
}
