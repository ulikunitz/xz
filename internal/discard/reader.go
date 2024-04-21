// Package discard supports optimized discarding of data from an io.Reader. It
// may use the Seek or Discard method of an underlying io.Reader.
package discard

import (
	"errors"
	"io"
	"math"
)

// Reader combines an io.Reader with a Discard64 method.
type Reader interface {
	io.Reader
	Discard64(n int64) (discarded int64, err error)
}

// simpleReader works on a pure io.Reader.
type simpleReader struct {
	io.Reader
	buf []byte
}

// newSimpleReader creates a [simpleReader] from an io.Reader.
func newSimpleReader(r io.Reader) *simpleReader {
	return &simpleReader{
		Reader: r,
		buf:    make([]byte, 16*1024),
	}
}

// Discard64 discards n bytes from the reader using the buffer of the
// simpleReader structure. It returns an error if n < 0 or if the underlying
// Read method returns an error.
func (d *simpleReader) Discard64(n int64) (discarded int64, err error) {
	if n <= 0 {
		if n < 0 {
			return 0, errors.New("discard: negative count")
		}
		return 0, nil
	}

	p := d.buf[:cap(d.buf)]
	k := n
	for k > 0 {
		if k < int64(len(p)) {
			p = p[:k]
		}
		r, err := d.Read(p)
		k -= int64(r)
		if err != nil {
			return n - k, err
		}
	}

	return n, nil
}

// readSeeker implements the Reader interface for an io.ReadSeeker.
type readSeeker struct {
	io.ReadSeeker
}

// newReadSeeker creates a [readSeeker] from an io.ReadSeeker.
func newReadSeeker(r io.ReadSeeker) (d *readSeeker, err error) {
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return &readSeeker{ReadSeeker: r}, nil
}

// Discard64 discards n bytes from the reader using the Seek method of the
// underlying   io.ReadSeeker. It returns an error if n < 0 or if the underlying
// Seek method returns an error.
func (d *readSeeker) Discard64(n int64) (discarded int64, err error) {
	if n <= 0 {
		if n < 0 {
			return 0, errors.New("discard: negative count")
		}
		return 0, nil
	}

	_, err = d.Seek(n, io.SeekCurrent)
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
	intReader
}

// newIntWrapper creates an [intWrapper] from an [intReader].
func newIntWrapper(r intReader) *intWrapper {
	return &intWrapper{intReader: r}
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
		d, err := w.Discard(math.MaxInt)
		discarded += int64(d)
		if err != nil {
			return discarded, err
		}
		n -= int64(d)
	}

	d, err := w.Discard(int(n))
	discarded += int64(d)
	return discarded, err
}

// Wrap wraps an io.Reader to implement the [Reader] interface with the
// Discard64 method. It will use a Seek method if supported and a possible
// Discard method.
func Wrap(r io.Reader) Reader {
	if rd, ok := r.(Reader); ok {
		return rd
	}
	if rs, ok := r.(io.ReadSeeker); ok {
		d, err := newReadSeeker(rs)
		if err == nil {
			return d
		}
	}
	if ir, ok := r.(intReader); ok {
		return newIntWrapper(ir)
	}
	return newSimpleReader(r)
}
