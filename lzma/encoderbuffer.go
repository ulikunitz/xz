package lzma

import (
	"errors"
	"fmt"
)

// encoderBuffer combines a circular byte buffer with a matcher.
type encoderBuffer struct {
	buffer
	matcher
}

// Write write the given data into the encoder buffer.
func (b *encoderBuffer) Write(p []byte) (n int, err error) {
	n, err = b.buffer.Write(p)
	k, merr := b.matcher.Write(p[:n])
	if merr != nil {
		panic(fmt.Errorf("matcher wrote %d of %d bytes because of %s",
			k, n, merr))
	}
	return
}

// Discard discards data from the encoder buffer. Data that has been
// discarded may be overwritten.
func (b *encoderBuffer) Discard(n int) (discarded int, err error) {
	return b.buffer.Discard(n)
}

// ReadByteAt allows extraction of a single byte from the encoder
// buffer. The position is the absolute offset of all data written to
// the encoder buffer.
func (b *encoderBuffer) ReadByteAt(pos int64) (c byte, err error) {
	d := b.Pos() - pos
	if !(0 < d && d <= int64(b.Buffered())) {
		return 0, errors.New("ReadByteAt: position not buffered")
	}
	i := b.front - int(d)
	if i < 0 {
		i += len(b.data)
	}
	return b.data[i], nil
}

// ReadAt reads data from a specific absolute position of the encoder
// buffer. The position gives the absolute offset of all data written to
// the buffer.
func (b *encoderBuffer) ReadAt(p []byte, pos int64) (n int, err error) {
	d := b.Pos() - pos
	if !(0 < d && d <= int64(b.Buffered())) {
		return 0, errors.New("ReadAt: position outside buffer")
	}
	n = int(d)
	if n < len(p) {
		p = p[:n]
		err = errors.New("ReadAt: insufficient data in buffer")
	}
	i := b.front - n
	if i < 0 {
		i += len(b.data)
	}
	k := copy(p, b.data[i:])
	if k < n {
		copy(p[k:], b.data)
	}
	return
}
