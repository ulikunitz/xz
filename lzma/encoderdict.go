package lzma

import (
	"errors"
	"fmt"
)

type encoderDict struct {
	buf      *encoderBuffer
	head     int64
	zero     int64
	capacity int
}

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

func initEncoderDict(e *encoderDict, dictCap int, buf *encoderBuffer) error {
	if !(minDictCap <= dictCap && dictCap <= maxDictCap) {
		return errors.New("initEncoderDict: dictCap out of range")
	}
	if !(dictCap <= buf.Cap()-MaxMatchLen) {
		return errors.New("initEncoderDict: bufCap too small")
	}
	_initEncoderDict(e, dictCap, buf)
	return nil
}

func (e *encoderDict) Reset() {
	e.head = 0
	e.zero = e.buf.Pos()
}

func (e *encoderDict) Pos() int64 {
	return e.zero + e.head
}

func (e *encoderDict) Len() int {
	if e.head >= int64(e.capacity) {
		return e.capacity
	}
	return int(e.head)
}

func (e *encoderDict) Buffered() int {
	return int(e.buf.Pos() - e.Pos())
}

func (e *encoderDict) Advance(n int) error {
	if !(0 < n && n <= e.Buffered()) {
		return errors.New("Advance: n out of range")
	}
	e.head += int64(n)
	return nil
}

func (e *encoderDict) ByteAt(distance int) byte {
	if !(0 < distance && distance <= e.Len()) {
		return 0
	}
	c, err := e.buf.ReadByteAt(e.Pos() - int64(distance))
	if err != nil {
		fmt.Printf("buf.Pos %d edict.Pos %d distance %d cap %d pos %d\n",
			e.buf.Pos(), e.Pos(), distance, e.capacity,
			e.Pos()-int64(distance))
		panic(fmt.Errorf("ByteAt: error %s", err))
	}
	return c
}

func (e *encoderDict) Literal() (b byte, err error) {
	return e.buf.ReadByteAt(e.Pos())
}

func (e *encoderDict) Matches() (distances []int, err error) {
	if e.Buffered() < e.buf.WordLen() {
		return nil, nil
	}
	hpos := e.Pos()
	p := make([]byte, e.buf.WordLen())
	if _, err = e.buf.ReadAt(p, hpos); err != nil {
		return nil, nil
	}
	positions, err := e.buf.matcher.Matches(p)
	if err != nil {
		return nil, err
	}
	n := int64(e.Len())
	for _, pos := range positions {
		d := hpos - pos
		if 0 < d && d <= n {
			distances = append(distances, int(d))
		}
	}
	return distances, nil
}

// MatchLen computes the length of the match at the given distance with
// the bytes at the head of the dictionary.. The function returns zero
// if no match is found.
func (e *encoderDict) MatchLen(dist int) int {
	if !(0 < dist && dist <= e.Len()) {
		return 0
	}
	b := e.Buffered()
	return e.buf.buffer.EqualBytes(b+dist, b, MaxMatchLen)
}
