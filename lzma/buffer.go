// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"fmt"
	"io"
)

// buffer provides a circular buffer. The type supports the io.Writer
// interface and other functions required to implement a dictionary.
//
// The top offset tracks the position of the buffer in the byte stream
// covered. The bottom offset marks the bottom of the buffer. The
// writeLimit marks the limit for additional writes.
//
// Change top only with the setTop function.
type buffer struct {
	data       []byte
	bottom     int64 // bottom == max(top - len(data), 0)
	top        int64
	writeLimit int64
}

// maxLimit provides the maximum value. Setting the writeLimit to
// this value disables the writeLimit for all practical purposes.
const maxLimit = 1<<63 - 1

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
		return nil, negError{"capacity", capacity}
	}
	c, err := toInt(capacity)
	if err != nil {
		return nil, lzmaError{fmt.Sprintf(
			"capacity %d cannot be represented as int", capacity)}
	}
	b = &buffer{data: make([]byte, c), writeLimit: maxLimit}
	return b, nil
}

// capacity returns the max)imum capacity of the buffer.
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
		panic(negError{"off", off})
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
	end := add(off, int64(len(p)))
	if end > b.writeLimit {
		m = int(b.writeLimit - off)
		p = p[:m]
		err = errWriteLimit
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
		return errWriteLimit
	}
	b.data[b.index(b.top)] = c
	b.setTop(b.top + 1)
	return nil
}

// writeSlice returns a slice from the buffer for direct writing of
// data. Note that depending of top in the ring buffer the array p might
// be smaller then end-buf.top.
func (b *buffer) writeSlice(end int64) (p []byte, err error) {
	if end <= b.top {
		if end < b.top {
			return nil, lzmaError{fmt.Sprintf("end=%d less than top=%d", end, b.top)}
		}
		return nil, errAgain
	}
	if end > b.writeLimit {
		return nil, errWriteLimit
	}
	s, e := b.index(b.top), b.index(end)
	if s < e {
		p = b.data[s:e]
	} else {
		p = b.data[s:]
	}
	return p, nil
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

// readByteAt returns the byte at the given offset. If off is not in the
// range [b.bottom, b.top) a rangeError is returned unless off equals
// b.top. In that case errAgain is returned.
func (b *buffer) readByteAt(off int64) (c byte, err error) {
	if !(b.bottom <= off && off < b.top) {
		if off == b.top {
			return 0, errAgain
		}
		return 0, rangeError{"offset", off}
	}
	c = b.data[b.index(off)]
	return c, nil
}

// writeRepAt writes a repetition into the buffer. Obviously the method is
// used to handle matches during decoding the LZMA stream.
func (b *buffer) writeRepAt(n int, off int64) (written int, err error) {
	if n < 0 {
		return 0, negError{"n", n}
	}
	if !(b.bottom <= off && off < b.top) {
		return 0, rangeError{"off", off}
	}

	start, end := off, add(off, int64(n))
	if end > b.writeLimit {
		return 0, errWriteLimit
	}
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
		return 0, rangeError{"off", off}
	}
	end := add(off, int64(len(p)))
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

// resets the buffer to its original values.
func (b *buffer) reset() {
	b.top = 0
	b.bottom = 0
	b.writeLimit = maxLimit
}
