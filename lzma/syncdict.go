// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

// syncDict provides a dictionary that is always synchronized with the
// top of the buffer. The field size provides the size of the
// dictionary. It must be less or equal the size of the buffer.
type syncDict struct {
	buf  *buffer
	size int64
}

// offset returns the current head of the dictionary, which is the top
// of the buffer.
func (sd *syncDict) offset() int64 {
	return sd.buf.top
}

// byteAt returns the the byte at the given distance.
func (sd *syncDict) byteAt(dist int64) byte {
	if !(0 < dist && dist <= sd.size) {
		panic("dist out of range")
	}
	off := sd.buf.top - dist
	if off < sd.buf.bottom {
		return 0
	}
	return sd.buf.data[sd.buf.index(off)]
}

// reset resets the dictionary and the buffer.
func (sd *syncDict) reset() {
	sd.buf.reset()
}

// writeRep writes a repetition to the top of the buffer and keeps the
// head of the dictionary synchronous with the buffer.
func (sd *syncDict) writeRep(dist int64, n int) (written int, err error) {
	if !(0 < dist && dist <= sd.size) {
		panic("dist out of range")
	}
	off := sd.buf.top - dist
	written, err = sd.buf.writeRepAt(n, off)
	return
}

// WriteByte writes a single byte into the buffer.
func (sd *syncDict) WriteByte(c byte) error {
	return sd.buf.WriteByte(c)
}

// newSyncDict creates a sync dictionary. The argument size defines the
// size of the dictionary. The capacity of the buffer is allowed to be
// larger.
func newSyncDict(buf *buffer, size int64) (sd *syncDict, err error) {
	if !(MinDictSize <= size && size <= int64(buf.capacity())) {
		return nil, rangeError{"size", size}
	}
	sd = &syncDict{buf: buf, size: size}
	return sd, nil
}
