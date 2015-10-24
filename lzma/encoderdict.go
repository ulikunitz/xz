package lzma

import (
	"errors"
	"fmt"
	"io"
)

// matcher is an interface that allows the identification of potential
// matches for words with a constant length greater or equal 2.
//
// The absolute offset of potential matches are provided by the
// Matches method. The current position of the matcher is provided by
// the Pos method.
//
// The Reset method clears the matcher completely but starts new data
// at the given position.
type matcher interface {
	io.Writer
	WordLen() int
	Pos() int64
	Matches(word []byte) (positions []int64)
	Reset()
}

// EncoderDict provides the dictionary for the encoder. It includes a
// matcher for searching matching strings in the dictionary. Note that
// the dictionary also supports a buffer of data that has yet to be
// moved into the dictionary.
type EncoderDict struct {
	buf      buffer
	m        matcher
	capacity int
}

// NewEncoderDict creates a new encoder dictionary. The initial position
// and length of the dictionary will be zero. There will be no buffered
// data.
func NewEncoderDict(dictCap, bufCap int) (d *EncoderDict, err error) {
	if !(minDictCap <= dictCap && dictCap <= maxDictCap) {
		return nil, errors.New(
			"lzma.NewEncoderDict: dictCap out of range")
	}
	if !(dictCap+maxMatchLen <= bufCap) {
		return nil, errors.New(
			"lzma.NewEncoderDict: buffer capacit not sufficient")
	}
	d = &EncoderDict{capacity: dictCap}
	if err = initBuffer(&d.buf, bufCap); err != nil {
		return nil, err
	}
	if d.m, err = newHashTable(dictCap, 4); err != nil {
		return nil, err
	}
	return d, nil
}

// Reset clears the dictionary. After return the state of the dictionary
// is the same as after NewEncoderDict.
func (d *EncoderDict) Reset() {
	d.buf.Reset()
	d.m.Reset()
}

// Available returns the number of bytes that can be written by a
// following Write call.
func (d *EncoderDict) Available() int {
	return d.buf.Available() - d.DictLen()
}

// Buffered gives the number of bytes available for a following Read or
// Advance.
func (d *EncoderDict) Buffered() int {
	return d.buf.Buffered()
}

// Len returns the number of bytes stored in the buffer.
func (d *EncoderDict) Len() int {
	n := d.m.Pos()
	a := int64(d.buf.Available())
	if n > a {
		return int(a)
	}
	return int(n)
}

// DictCap returns the dictionary capacity.
func (d *EncoderDict) DictCap() int {
	return d.capacity
}

// BufCap returns the buffer capacity.
func (d *EncoderDict) BufCap() int {
	return d.buf.Cap()
}

// DictLen returns the current number of bytes of the dictionary. The
// number has dictCap as upper limit.
func (d *EncoderDict) DictLen() int {
	n := d.m.Pos()
	if n > int64(d.capacity) {
		return d.capacity
	}
	return int(n)
}

// Pos returns the current position of the dictionary head.
func (d *EncoderDict) Pos() int64 {
	return d.m.Pos()
}

// ByteAt returns a byte from the dictionary. The distance is the
// positive difference from the current head. A distance of 1 will
// return the top-most byte in the dictionary.
func (d *EncoderDict) ByteAt(distance int) byte {
	if !(0 < distance && distance < d.Len()) {
		return 0
	}
	i := d.buf.rear - distance
	if i < 0 {
		i += len(d.buf.data)
	}
	return d.buf.data[i]
}

// Write puts new data into the dictionary.
func (d *EncoderDict) Write(p []byte) (n int, err error) {
	n = len(p)
	m := d.Available()
	if n > m {
		p = p[:m]
		err = errNoSpace
	}
	var werr error
	n, werr = d.buf.Write(p)
	if werr != nil {
		err = werr
	}
	return n, err
}

// Read reads data from the buffer in front of the dictionary. Reading
// has the same effect as Advance on the dictionary.
func (d *EncoderDict) Read(p []byte) (n int, err error) {
	n, err = d.buf.Peek(p)
	p = p[:n]
	var cerr error
	if n, cerr = d.Advance(n); cerr != nil {
		err = cerr
	}
	return n, err
}

// Advance moves the dictionary head ahead by the given number of bytes.
func (d *EncoderDict) Advance(n int) (advanced int, err error) {
	written, err := io.CopyN(d.m, &d.buf, int64(n))
	return int(written), err
}

// CopyN copies the n topmost bytes of the dictionary. The maximum for n
// is given by the Len() method.
func (d *EncoderDict) CopyN(w io.Writer, n int64) (written int64, err error) {
	m := int64(d.Len())
	if n > m {
		n = m
	}
	if n <= 0 {
		return 0, nil
	}
	var k int
	i := d.buf.rear - int(n)
	if i >= 0 {
		k, err = w.Write(d.buf.data[i:d.buf.rear])
		return int64(k), err
	}
	i += len(d.buf.data)
	k, err = w.Write(d.buf.data[i:])
	written = int64(k)
	if err != nil {
		return written, err
	}
	k, err = w.Write(d.buf.data[:d.buf.rear])
	written += int64(k)
	return written, err
}

// Literal returns the the byte at the position of the head. The method
// returns 0 if no bytes are buffered.
func (d *EncoderDict) Literal() byte {
	if d.buf.rear == d.buf.front {
		return 0
	}
	return d.buf.data[d.buf.rear]
}

// Matches returns potential distances for the word at the head of the
// dictionary. If there are not enough bytes a nil slice will be
// returned.
func (d *EncoderDict) Matches() (distances []int) {
	w := d.m.WordLen()
	if d.buf.Buffered() < w {
		return nil
	}
	p := make([]byte, w)
	// Peek doesn't return errors and we have ensured that there are
	// enough bytes.
	d.buf.Peek(p)
	positions := d.m.Matches(p)
	n := int64(d.DictLen())
	hpos := d.m.Pos()
	for _, pos := range positions {
		d := hpos - pos
		if 0 < d && d <= n {
			distances = append(distances, int(d))
		}
	}
	return distances
}

// MatchLen computes the length of the match at the given distance with
// the bytes at the head of the dictionary. The function returns zero
// if no match is found.
func (d *EncoderDict) MatchLen(dist int) int {
	if !(0 < dist && dist <= d.DictLen()) {
		return 0
	}
	b := d.Buffered()
	return d.buf.EqualBytes(b+dist, b, maxMatchLen)
}
