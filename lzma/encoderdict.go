package lzma

import (
	"errors"
	"fmt"
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
