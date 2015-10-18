package lzma

import (
	"errors"
	"fmt"
	"io"
)

// encoderDict provides the dictionary for the encoder.
type encoderDict struct {
	buf *encoderBuffer
	// offset of bytes to the zero value
	head int64
	// zero position in terms of the absolute position of the
	// encoder buffer
	zero int64
	// dictionary capacity
	capacity int
}

// _initEncoderDict initializes the encoder without checking the dictCap
// value. This allows small dictionary for thesting.
func _initEncoderDict(e *encoderDict, dictCap int, buf *encoderBuffer) {

	if buf == nil {
		panic("_initEncoderDict: buf must not be nil")
	}
	*e = encoderDict{
		buf:      buf,
		zero:     buf.Pos(),
		capacity: dictCap,
	}
}

// initEncoderDict initializes the encoder dictionary.
func initEncoderDict(e *encoderDict, dictCap int, buf *encoderBuffer) error {
	if !(minDictCap <= dictCap && dictCap <= maxDictCap) {
		return errors.New("initEncoderDict: dictCap out of range")
	}
	if !(dictCap <= buf.Cap()-maxMatchLen) {
		return errors.New("initEncoderDict: bufCap too small")
	}
	_initEncoderDict(e, dictCap, buf)
	return nil
}

// Reset resets the dictionary. After the method the dictionary will
// have length zero. The buffer will not be changed.
func (e *encoderDict) Reset() {
	e.head = 0
	e.zero = e.buf.Pos()
}

// Pos gives the absolute position of the dictionary head for all data
// written to the encoder buffer.
func (e *encoderDict) Pos() int64 {
	return e.zero + e.head
}

// Len returns the current amount of data in the dictionary.
func (e *encoderDict) Len() int {
	if e.head >= int64(e.capacity) {
		return e.capacity
	}
	return int(e.head)
}

// Buffered returns the number of bytes available before the head of the
// dictionary.
func (e *encoderDict) Buffered() int {
	return int(e.buf.Pos() - e.Pos())
}

// Advance the dictionary head by n bytes.
func (e *encoderDict) Advance(n int) {
	if !(0 < n && n <= e.Buffered()) {
		panic(errors.New("Advance: n out of range"))
	}
	e.head += int64(n)
}

// ByteAt returns a byte from the dictionary. The distance is the
// positiove value to the head.
func (e *encoderDict) ByteAt(distance int) byte {
	if !(0 < distance && distance <= e.Len()) {
		return 0
	}
	c, err := e.buf.ReadByteAt(e.Pos() - int64(distance))
	if err != nil {
		panic(fmt.Errorf("ByteAt: error %s", err))
	}
	return c
}

// Literal returns the the byte at the position of the head.
func (e *encoderDict) Literal() byte {
	c, err := e.buf.ReadByteAt(e.Pos())
	if err != nil {
		panic(fmt.Errorf("Literal: %s", err))
	}
	return c
}

// Matches returns potential distances for the word at the head of the
// dictionary. If there are not enough characters a nil slice will be
// returned.
func (e *encoderDict) Matches() (distances []int) {
	if e.Buffered() < e.buf.WordLen() {
		return nil
	}
	hpos := e.Pos()
	p := make([]byte, e.buf.WordLen())
	if _, err := e.buf.ReadAt(p, hpos); err != nil {
		return nil
	}
	positions := e.buf.matcher.Matches(p)
	n := int64(e.Len())
	for _, pos := range positions {
		d := hpos - pos
		if 0 < d && d <= n {
			distances = append(distances, int(d))
		}
	}
	return distances
}

// MatchLen computes the length of the match at the given distance with
// the bytes at the head of the dictionary.. The function returns zero
// if no match is found.
func (e *encoderDict) MatchLen(dist int) int {
	if !(0 < dist && dist <= e.Len()) {
		return 0
	}
	b := e.Buffered()
	return e.buf.buffer.EqualBytes(b+dist, b, maxMatchLen)
}

// CopyN copies the last n bytes stored in the dictionary. It is an
// error if n exceeds the number of bytes stored in the dictionary.
func (e *encoderDict) CopyN(w io.Writer, n int) (written int, err error) {
	buf := e.buf.buffer
	if n > buf.Buffered() {
		return 0, errors.New(
			"encoderDict.CopyN: not enough data in dictionary")
	}
	buf.rear = buf.front - n
	if buf.rear < 0 {
		buf.rear += len(buf.data)
	}
	k, err := io.CopyN(w, &buf, int64(n))
	return int(k), err
}

// TODO: New EncoderDict type for the new simplified encoder.

// EncoderDict provides the dictionary for the encoder. It includes a
// matcher for searching matching strings in the dictionary. Note that
// the dictionary also supports a buffer of data that has yet to be
// moved into the dictionary.
type EncoderDict struct {
	buf      buffer
	m        matcher
	head     int
	capacity int
}

// Creates a new encoder dictionary. The initial position and length of
// the dictionary will be zero. There will be no buffered data.
func NewEncoderDict(dictCap, bufCap int) (ed *EncoderDict, err error) {
	panic("TODO")
}

// Resets the dictionary and sets the position to the given value. The
// dictionary the will be cleared and the buffered data maybe discarded
// based on the value of pos.
func (ed *EncoderDict) Reset(pos int64) error {
	panic("TODO")
}

// Available returns the number of bytes that can be buffered by a
// following Write call.
func (ed *EncoderDict) Available() int {
	panic("TODO")
}

// Buffered gives the number of bytes available for a following Read or
// Advance.
func (ed *EncoderDict) Buffered() int {
	panic("TODO")
}

// Len returns the number of bytes stored in the dictionary after the
// current position. It may be larger then the dictionary length.
func (ed *EncoderDict) Len() int {
	panic("TODO")
}

// DictCap returns the dictionary capacity.
func (ed *EncoderDict) DictCap() int {
	return ed.capacity
}

// BufCap returns the buffer capacity.
func (ed *EncoderDict) BufCap() int {
	panic("TODO")
}

// DictLen returns the current number of bytes of the dictionary. The
// number has dictCap as upper limit.
func (ed *EncoderDict) DictLen() int {
	n := ed.Len()
	if n > ed.capacity {
		return ed.capacity
	}
	return n
}

// Returns the current position of the dictionary head.
func (ed *EncoderDict) Pos() int64 {
	panic("TODO")
}

// ByteAt returns a byte from the dictionary. The distance is the
// positive difference from the current head. A distance of 1 will
// return the top-most byte in the dictionary.
func (ed *EncoderDict) ByteAt(distance int) byte {
	panic("TODO")
}

// Write puts new data into the dictionary.
func (ed *EncoderDict) Write(p []byte) (n int, err error) {
	panic("TODO")
}

// Read reads data from the buffer in front of the dictionary. Reading
// has the same effect as Advance on the dictionary.
func (ed *EncoderDict) Read(p []byte) (n int, err error) {
	panic("TODO")
}

// Advance moves the dictionary head ahead by the given number of bytes.
func (ed *EncoderDict) Advance(n int) (advanced int, err error) {
	panic("TODO")
}

// CopyN copies the n topmost bytes of the dictionary. The maximum for n
// is given by the Len() method.
func (ed *EncoderDict) CopyN(w io.Writer, n int64) (written int64, err error) {
	panic("TODO")
}

// Discard discards data at the end of the dictionary buffer.
func (ed *EncoderDict) Discard(n int) (discarded int, err error) {
	panic("TODO")
}
