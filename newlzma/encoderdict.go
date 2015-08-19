package newlzma

import (
	"errors"
	"io"
)

// matcher is an interface that allows the identification of potential
// matches for words greater or equal 2. A matcher supports only words
// of a specific length.
//
// The content of the dictionary must be provided by the Write method.
//
// Potential matches for short words of a specific word length are
// provided by the Matches method.
type matcher interface {
	io.Writer
	WordLen() int
	Matches(word []byte) (distances []int, err error)
}

// encoderDict provides the dictionary for encoding. The buffer is used
// for providing the data to the encoder. The head of the dictionary is
// at the rear of the circular buffer.
type encoderDict struct {
	buf buffer
	// position of the dictionary head; the head is at the rear of
	// the buffer
	head int64
	// capacity of then dictionary
	capacity int
	// matcher
	matcher matcher
}

// _initEncoderDict initializes the encoder allowing initializations
// for testing with small sizes.
func _initEncoderDict(e *encoderDict, dictCap, bufCap int, m matcher) error {
	*e = encoderDict{capacity: dictCap, matcher: m}
	return initBuffer(&e.buf, bufCap)
}

// initializes the encoder dictionary. Note that bufCap must be at
// least maxMatchLen(273) bytes larger than the dictionary capacity
// dictCap.
func initEncoderDict(e *encoderDict, dictCap, bufCap int, m matcher) error {
	if !(minDictCap <= dictCap && dictCap <= maxDictCap) {
		return errors.New("initEncoderDict: dictCap out of range")
	}
	if !(dictCap <= bufCap-maxMatchLen) {
		return errors.New("initEncoderDict: bufCap too small")
	}
	if m == nil {
		return errors.New("matcher m is nil")
	}
	return _initEncoderDict(e, dictCap, bufCap, m)
}

// Head returns the dictionary head value.
func (e *encoderDict) Head() int64 { return e.head }

// Len returns the current length of the dictionary.
func (e *encoderDict) Len() int {
	if e.head >= int64(e.capacity) {
		return e.capacity
	}
	return int(e.head)
}

// ByteAt returne a byte stored in the dictionary. If the distance is
// non-positive or exceeds the current length of the dictionary the zero
// byte is provided.
func (e *encoderDict) ByteAt(dist int) byte {
	if !(0 < dist && dist <= e.Len()) {
		return 0
	}
	i := e.buf.rear - dist
	if i < 0 {
		i += len(e.buf.data)
	}
	return e.buf.data[i]
}

// Available returns the number of bytes available for Writing. Note
// that a write is not allowed to overwrite the dictionary.
func (e *encoderDict) Available() int {
	return e.buf.Available() - e.Len()
}

// Buffered returns the number of bytes buffered that are yet not part
// of the dictionary.
func (e *encoderDict) Buffered() int {
	return e.buf.Buffered()
}

// Advance moves the dictionary head n bytes ahead. Data will be written
// into the matcher.
func (e *encoderDict) Advance(n int) error {
	if !(0 <= n && n <= e.buf.Buffered()) {
		return errors.New("Advance: n out of range")
	}
	// Optimize(uk): transfer array directly from data to matcher
	if _, err := io.CopyN(e.matcher, &e.buf, int64(n)); err != nil {
		return nil
	}
	e.head += int64(n)
	return nil
}

// Matches find potential matches for the current dictionary head.
func (e *encoderDict) Matches() (distances []int, err error) {
	p := make([]byte, e.matcher.WordLen())
	n, err := e.buf.Peek(p)
	if err != nil {
		return nil, err
	}
	if n != len(p) {
		return nil, errors.New("Matches: not enough bytes buffered")
	}
	return e.matcher.Matches(p)
}

// MatchLen computes the length of the match at the given distance with
// the current head of the buffer.
// function returns zero if no match is found.
func (e *encoderDict) MatchLen(dist int) int {
	if !(0 < dist && dist <= e.Len()) {
		return 0
	}
	i := e.buf.rear - dist
	if i < 0 {
		i += len(e.buf.data)
	}
	j := e.buf.rear
	m := e.Buffered()
	if maxMatchLen < m {
		m = maxMatchLen
	}
	for n := 0; n < m; n++ {
		if e.buf.data[i] != e.buf.data[j] {
			return n
		}
		i = e.buf.addIndex(i, 1)
		j = e.buf.addIndex(j, 1)
	}
	return m
}

// Write adds new data into the buffer. The position of the dictionary
// and any related status will not be changed.
func (e *encoderDict) Write(p []byte) (n int, err error) {
	m := e.Available()
	if len(p) > m {
		p = p[:m]
	}
	return e.buf.Write(p)
}

// Reset clears the dictionary. The current length of the dictionary
// will be zero as well as head value.
func (e *encoderDict) Reset() {
	e.head = 0
}
