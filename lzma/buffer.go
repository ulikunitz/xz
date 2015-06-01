package lzma

import (
	"errors"
	"io"
)

// buffer provides a circular buffer. The type supports the io.Writer
// interface and other functions required to implement a dictionary.
//
// The top offset tracks the position of the buffer in the byte stream
// covered. The bottom offset marks the bottom of the buffer. The
// writeLimit marks the limit for additional writes.
type buffer struct {
	data       []byte
	bottom     int64 // bottom == max(top - len(data), 0)
	top        int64
	writeLimit int64
}

// maxLimit provides the maximum value. Setting the writeLimit to
// this value disables the writeLimit for all practical purposes.
const maxLimit = 1<<63 - 1

// Errors returned by buffer methods.
var (
	errOffset = errors.New("offset outside buffer range")
	errAgain  = errors.New("buffer overflow; repeat")
	errNegLen = errors.New("length is negative")
	errLimit  = errors.New("write limit reached")
	errCap    = errors.New("capacity out of range")
	errInt64  = errors.New("int64 values not representable as int")
)

// toInt converts an int64 value to an int value. If the number is not
// representable as int, errInt64 is returned.
func toInt(n int64) (int, error) {
	k := int(n)
	if int64(k) != n {
		return 0, errInt64
	}
	return k, nil
}

// newBuffer creates a new buffer.
func newBuffer(capacity int64) (b *buffer, err error) {
	if capacity < 0 {
		return nil, errCap
	}
	c, err := toInt(capacity)
	if err != nil {
		return nil, errCap
	}
	b = &buffer{data: make([]byte, c), writeLimit: maxLimit}
	return b, nil
}

// capacity returns the maximum capacity of the buffer.
func (b *buffer) capacity() int {
	return len(b.data)
}

// length returns the actual length of the buffer.
func (b *buffer) length() int {
	return int(b.top - b.bottom)
}

// setTop sets the top and bottom offset. Any modification of the top
// offset must use this method.
func (b *buffer) setTop(off int64) {
	if off < 0 {
		panic("b.Top overflow?")
	}
	if off > b.writeLimit {
		panic("off exceeds writeLimit")
	}
	b.top = off
	b.bottom = off - int64(len(b.data))
	if b.bottom < 0 {
		b.bottom = 0
	}
}

// index converts a byte stream offset into an index of the data field.
func (b *buffer) index(off int64) int {
	if off < 0 {
		panic("negative offset?")
	}
	return int(off % int64(len(b.data)))
}

// Write writes a byte slice into the buffer. It satisfies the io.Write
// interface.
func (b *buffer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}
	var m int
	off := b.top
	if off+int64(len(p)) > b.writeLimit {
		m = int(b.writeLimit - off)
		p = p[:m]
		err = errLimit
	}
	m = len(p) - len(b.data)
	if m > 0 {
		off += int64(m)
		p = p[m:]
	}
	for len(p) > 0 {
		m = copy(b.data[b.index(off):], p)
		off += int64(m)
		p = p[m:]
	}
	n = int(off - b.top)
	b.setTop(off)
	return n, err
}

// WriteByte writes a single byte into the buffer. The method satisfies
// the io.ByteWriter interface.
func (b *buffer) WriteByte(c byte) error {
	if b.top >= b.writeLimit {
		return errLimit
	}
	b.data[b.index(b.top)] = c
	b.setTop(b.top + 1)
	return nil
}

// writeRangeTo is a helper function that writes all data between off
// and end to the writer. The function doesn't check the arguments.
func (b *buffer) writeRangeTo(off, end int64, w io.Writer) (written int, err error) {
	// assume that arguments are correct
	start := off
	e := b.index(end)
	for off < end {
		s := b.index(off)
		var q []byte
		if s < e {
			q = b.data[s:e]
		} else {
			q = b.data[s:]
		}
		var k int
		k, err = w.Write(q)
		off += int64(k)
		if err != nil {
			break
		}
	}
	return int(off - start), err
}

// writeRepAt writes a repetition into the buffer. Obviously the method is
// used to handle matches during decoding the LZMA stream.
func (b *buffer) writeRepAt(n int, off int64) (written int, err error) {
	if n < 0 {
		return 0, errNegLen
	}
	if !(b.bottom <= off && off < b.top) {
		return 0, errOffset
	}

	start, end := off, off+int64(n)
	for off < end {
		var next int64
		if b.top < end {
			next = b.top
		} else {
			next = end
		}
		var k int
		k, err = b.writeRangeTo(off, next, b)
		off += int64(k)
		if err != nil {
			break
		}
	}
	return int(off - start), err
}

// readAtBuffer provides a wrapper for the p buffer of the ReadAt
// function.
type readAtBuffer struct {
	p []byte
}

// errSpace indicates insufficient space in the readAtBuffer.
var errSpace = errors.New("out of buffer space")

// Write satisfies the Writer interface for readAtBuffer.
func (b *readAtBuffer) Write(p []byte) (n int, err error) {
	n = copy(b.p, p)
	b.p = b.p[n:]
	if n < len(p) {
		err = errSpace
	}
	return
}

// ReadAt provides the ReaderAt interface.
func (b *buffer) ReadAt(p []byte, off int64) (n int, err error) {
	if !(b.bottom <= off && off <= b.top) {
		return 0, errOffset
	}
	end := off + int64(len(p))
	if end > b.top {
		end = b.top
	}
	n, err = b.writeRangeTo(off, end, &readAtBuffer{p})
	if err == errSpace {
		err = nil
	}
	return
}

// equalBytes count the equal bytes at off1 and off2 until max is reached.
func (b *buffer) equalBytes(off1, off2 int64, max int) int {
	if off1 < b.bottom || off2 < b.bottom || max <= 0 {
		return 0
	}
	m := int64(max)
	k := b.top - off1
	if k < m {
		if k <= 0 {
			return 0
		}
		m = k
	}
	k = b.top - off2
	if k < m {
		if k <= 0 {
			return 0
		}
		m = k
	}
	for k = 0; k < m; k++ {
		i, j := b.index(off1+k), b.index(off2+k)
		if b.data[i] != b.data[j] {
			break
		}
	}
	return int(k)
}

func (b *buffer) reset() {
	b.top = 0
	b.bottom = 0
	b.writeLimit = maxLimit
}
